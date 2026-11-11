// Package profilepage fetches a public Instagram profile by scraping the
// logged-out, server-rendered profile page (https://www.instagram.com/{user}/)
// through the configured proxy.
//
// It exists because the old i.instagram.com/api/v1/users/web_profile_info/
// REST endpoint (see package webprofile, left untouched) now returns a genuine
// 404 to unauthenticated requests. The profile HTML page, by contrast, still
// embeds the full profile JSON (new XDTUserDict schema) when the request:
//   - goes through a residential proxy (a datacenter/server IP gets a 302), and
//   - carries logged-out cookies (datr/mid/csrftoken/ig_did), which we mint by
//     first hitting the homepage, and browser "navigate" headers.
//
// Instagram serves variable HTML to logged-out clients: a full SSR page (with
// embedded JSON and og:description), an embedded-JSON-only variant, or a
// degraded shell (<title>Instagram</title>, no data) when the exit IP is not
// trusted. Because Bright Data rotates its exit IP per request, we simply retry
// the fetch a few times until a usable page comes back.
//
// GetWebProfile mirrors webprofile.GetWebProfile's signature so the router can
// switch to it without any other change.
package profilepage

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

const (
	browserUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	maxAttempts = 4
)

// errShellOnly means every attempt on a proxy returned Instagram's generic
// logged-out shell (no profile data, no not-found page). A profile that exists
// renders server-side reliably across fresh residential IPs, so when every
// proxy reports this the account almost certainly does not exist.
var errShellOnly = errors.New("profilepage: only degraded shell returned")

// GetWebProfile returns a public profile for username. It tries PROXY first and
// PROXY_FALLBACK next (same env vars as package webprofile).
//
// Return contract (matches how UsernameHandler consumes it):
//   - found:          profile populated, statusCode 200, err nil
//   - no such user:   statusCode 404, err nil (real not-found page, or only
//     shells on every proxy — see errShellOnly)
//   - transient block: err != nil (so the caller does NOT cache it and can retry)
func GetWebProfile(username string) (profile instagram.Profile, statusCode int, isRestricted bool, err error) {
	proxies := []string{os.Getenv("PROXY")}
	if fb := os.Getenv("PROXY_FALLBACK"); fb != "" {
		proxies = append(proxies, fb)
	}

	allShell := true
	for i, p := range proxies {
		profile, statusCode, isRestricted, err = getViaProxy(username, p)
		if err == nil {
			return // data (200) or a real not-found page (404)
		}
		if !errors.Is(err, errShellOnly) {
			allShell = false
		}
		log.Println("profilepage: proxy", i, "failed:", err)
	}
	if allShell {
		// Only the generic shell on every proxy across all retries: the account
		// almost certainly does not exist. Report 404 so the caller returns
		// "not exist" and caches it, instead of a retried system error.
		log.Println("profilepage:", username, "only shells on all proxies — treating as not found")
		return instagram.Profile{IsExist: "no"}, 404, false, nil
	}
	return
}

func getViaProxy(username, proxy string) (profile instagram.Profile, statusCode int, isRestricted bool, err error) {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if proxy != "" {
		pu, perr := url.Parse(proxy)
		if perr != nil {
			return profile, 0, false, perr
		}
		transport.Proxy = http.ProxyURL(pu)
		log.Println("profilepage: using proxy:", pu.Redacted()) // Redacted hides credentials
	}
	client := &http.Client{Transport: transport, Jar: jar, Timeout: 60 * time.Second}

	// Mint logged-out cookies (datr/mid/csrftoken/ig_did); Instagram serves the
	// full SSR profile page only to a request that already carries them.
	if _, _, e := doGet(client, "https://www.instagram.com/", false); e != nil {
		log.Println("profilepage: cookie bootstrap failed (continuing):", e)
	}

	// The post count lives only in og:description; the embedded JSON carries
	// everything else but no profile media count. Instagram serves og
	// inconsistently, so prefer an og-bearing variant, but keep an embedded-only
	// page as a fallback rather than lose follower/bio over a missing og.
	profileURL := "https://www.instagram.com/" + url.PathEscape(username) + "/"
	var (
		best           instagram.Profile
		bestRestricted bool
		haveBest       bool
		sawShell       bool
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, code, e := doGet(client, profileURL, true)
		if e != nil {
			err = e
			continue // transient network/proxy error — retry
		}
		statusCode = code

		if code == 404 {
			profile.IsExist = "no"
			return profile, 404, false, nil
		}
		if code != 200 {
			// 302 (server IP), 429, 5xx — transient, retry.
			err = fmt.Errorf("profilepage: status %d for %s", code, username)
			continue
		}

		html := string(body)
		if isNotFoundPage(html) {
			profile.IsExist = "no"
			return profile, 404, false, nil
		}
		if reOgDesc.MatchString(html) {
			// Richest variant: embedded JSON (usually) plus the post count.
			profile, isRestricted = parseProfilePage(html, username)
			return profile, 200, isRestricted, nil
		}
		if reFollower.MatchString(html) {
			// Embedded-only: keep it, but retry for an og variant with posts.
			best, bestRestricted = parseProfilePage(html, username)
			haveBest = true
			err = fmt.Errorf("profilepage: no og:description for %s (attempt %d)", username, attempt)
			continue
		}
		// Degraded shell (no data, not a not-found page) — retry a fresh IP.
		sawShell = true
		err = fmt.Errorf("profilepage: degraded shell for %s (attempt %d)", username, attempt)
	}
	if haveBest {
		return best, 200, bestRestricted, nil // embedded data; post count may be 0
	}
	if sawShell {
		// Every 200 response was the generic shell. Signal "likely nonexistent"
		// so GetWebProfile can decide across all proxies.
		return profile, statusCode, false, errShellOnly
	}
	// Only transient network/proxy errors (no page seen at all) — surface them
	// so the caller does not cache anything and can retry.
	if err == nil {
		err = errors.New("profilepage: no usable page for " + username)
	}
	return profile, statusCode, false, err
}

func doGet(client *http.Client, address string, navigate bool) ([]byte, int, error) {
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if navigate {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

var (
	reFollower  = regexp.MustCompile(`"follower_count":(\d+)`)
	reFollowing = regexp.MustCompile(`"following_count":(\d+)`)
	reFullName  = regexp.MustCompile(`"full_name":("(?:[^"\\]|\\.)*")`)
	reBio       = regexp.MustCompile(`"biography":("(?:[^"\\]|\\.)*")`)
	rePic       = regexp.MustCompile(`"profile_pic_url":("(?:[^"\\]|\\.)*")`)
	reExternal  = regexp.MustCompile(`"external_url":("(?:[^"\\]|\\.)*")`)
	rePk        = regexp.MustCompile(`"pk":"(\d+)"`)
	reVerified  = regexp.MustCompile(`"is_verified":(true|false)`)
	rePrivate   = regexp.MustCompile(`"is_private":(true|false)`)
	reOgDesc    = regexp.MustCompile(`property="og:description" content="([^"]*)"`)
	reOgImage   = regexp.MustCompile(`property="og:image" content="([^"]*)"`)
	reOgPosts   = regexp.MustCompile(`([\d.,]+)\s+Posts`)
	reOgFollow  = regexp.MustCompile(`([\d.,]+[KMB]?)\s+Followers`)
	reOgFollowing = regexp.MustCompile(`([\d.,]+[KMB]?)\s+Following`)
	reOgName    = regexp.MustCompile(`Posts - (.+?) \(@`)
)

// isNotFoundPage reports whether the HTML is Instagram's "this user does not
// exist" page (served with HTTP 200).
func isNotFoundPage(html string) bool {
	return strings.Contains(html, "Page Not Found") ||
		strings.Contains(html, "Sorry, this page isn't available") ||
		strings.Contains(html, "isn't available on Instagram")
}

// parseProfilePage extracts the profile. Follower/following/bio/verified/private
// come from the embedded JSON when present (exact); the post count comes from
// og:description (Instagram no longer serves media_count to logged-out
// requests); og:description also provides rounded fallbacks when the embedded
// JSON is missing.
func parseProfilePage(html, username string) (p instagram.Profile, isRestricted bool) {
	hasJSON := reFollower.MatchString(html)
	og := firstStr(reOgDesc, html)

	p.Follower = firstInt(reFollower, html)
	p.Following = firstInt(reFollowing, html)
	p.FullName = firstJSONStr(reFullName, html)
	p.Biography = firstJSONStr(reBio, html)
	p.ProfilePicUrl = firstJSONStr(rePic, html)
	p.ExternalURL = firstJSONStr(reExternal, html)
	p.ID = firstStr(rePk, html)
	p.IsVerified = firstStr(reVerified, html) == "true"
	p.IsPrivate = firstStr(rePrivate, html) == "true"

	if og != "" {
		if m := reOgPosts.FindStringSubmatch(og); m != nil {
			p.TotalPost = atoiLoose(m[1])
		}
		if p.Follower == 0 {
			if m := reOgFollow.FindStringSubmatch(og); m != nil {
				p.Follower = parseCount(m[1])
			}
		}
		if p.Following == 0 {
			if m := reOgFollowing.FindStringSubmatch(og); m != nil {
				p.Following = parseCount(m[1])
			}
		}
		if p.FullName == "" {
			if m := reOgName.FindStringSubmatch(og); m != nil {
				p.FullName = htmlUnescape(strings.TrimSpace(m[1]))
			}
		}
	}
	if p.ProfilePicUrl == "" {
		p.ProfilePicUrl = htmlUnescape(firstStr(reOgImage, html))
	}

	if hasJSON || og != "" {
		p.Username = username
		p.IsExist = "yes"
	}
	return p, isRestricted
}

func firstStr(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func firstInt(re *regexp.Regexp, s string) int {
	n, _ := strconv.Atoi(firstStr(re, s))
	return n
}

// firstJSONStr captures a JSON string token (with its quotes) and decodes its
// escapes (\uXXXX, \n, surrogate pairs, …) via the JSON unmarshaller.
func firstJSONStr(re *regexp.Regexp, s string) string {
	raw := firstStr(re, s)
	if raw == "" {
		return ""
	}
	var out string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return ""
	}
	return out
}

// atoiLoose parses "1,801" / "1.801" (thousands separators) into 1801.
func atoiLoose(s string) int {
	s = strings.NewReplacer(",", "", ".", "", " ", "").Replace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// parseCount parses og:description counts, which may carry a K/M/B suffix
// ("9M", "10.3K", "1,234"). Suffixed values are rounded by Instagram, so this
// is only a fallback for when the exact embedded count is absent.
func parseCount(s string) int {
	s = strings.TrimSpace(s)
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "K"):
		mult, s = 1e3, strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult, s = 1e6, strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "B"):
		mult, s = 1e9, strings.TrimSuffix(s, "B")
	}
	if mult == 1 {
		return atoiLoose(s)
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil {
		return 0
	}
	return int(f * mult)
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&#064;", "@", "&#x2F;", "/", "&quot;", `"`, "&#39;", "'").Replace(s)
}
