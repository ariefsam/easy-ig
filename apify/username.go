package apify

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/patrickmn/go-cache"
	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

var c = cache.New(5*time.Minute, 10*time.Minute)

func Username(username string) (data instagram.Profile, err error) {
	if v, ok := c.Get(username); ok {
		data = v.(instagram.Profile)
		log.Println("from cache", username)
		return
	}

	data, err = UsernameWithoutCache(username)
	if err != nil {
		log.Println(err)
		return
	}

	c.Set(username, data, cache.DefaultExpiration)

	return
}

func UsernameWithoutCache(username string) (data instagram.Profile, err error) {
	wait := make(chan instagram.Profile)
	inputChan <- responseUsername{
		name: username,
		wait: wait,
	}
	data = <-wait
	log.Println("done username", username)
	return
}

type responseUsername struct {
	name string
	wait chan instagram.Profile
}

var inputChan = make(chan responseUsername, 100000)

func init() {
	go usernameWorker()
}

func usernameWorker() {
	waiters := []responseUsername{}
	username := map[string]bool{}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	for {
		var mustRun bool

		select {
		case input := <-inputChan:
			log.Println("input", input)
			waiters = append(waiters, input)

			username[input.name] = true
			if len(waiters) >= 20 {
				mustRun = true
			}
		case <-ctx.Done():
			log.Println("tick")
			mustRun = true
			ctx, _ = context.WithTimeout(context.Background(), time.Second*5)
		}

		if len(username) == 0 {
			continue
		}

		if !mustRun {
			continue
		}

		usernames := []string{}
		for k := range username {
			usernames = append(usernames, k)
		}

		username = map[string]bool{}

		copyWaiters := []responseUsername{}
		for _, w := range waiters {
			copyWaiters = append(copyWaiters, w)
		}
		waiters = []responseUsername{}

		time.Sleep(10 * time.Millisecond)

		go executeUsername(copyWaiters, usernames)
	}

}

func executeUsername(waiters []responseUsername, usernames []string) {
	log.Println("execute", usernames)
	responseAll, err := execute(usernames)
	if err != nil {
		log.Println(err)

	}
	log.Println("done")

	log.Println(string(responseAll))

	responseClean, _, err := transform(responseAll)
	if err != nil {
		log.Println(err)
	}

	temp, _ := json.MarshalIndent(responseClean, "", "  ")
	log.Println(string(temp))

	respByUsername := map[string]instagram.Profile{}
	for _, v := range responseClean {
		respByUsername[v.Username] = v
	}

	for _, w := range waiters {
		w.wait <- respByUsername[w.name]
	}
}

// func executeByWebProfile(waiters []responseUsername, usernames []string) {
// 	data, _, _, err = GetWebProfile(username)
// 	return
// }
