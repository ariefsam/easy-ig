package apify

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"gitlab.com/ariefhidayatulloh/easy-ig/instagram"
)

func Username(username string) (data instagram.Profile, err error) {
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

var inputChan = make(chan responseUsername)

func init() {
	go usernameWorker()
}

func usernameWorker() {
	waiters := []responseUsername{}
	username := map[string]bool{}

	for {
		var mustRun bool
		ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
		select {
		case input := <-inputChan:
			log.Println("input", input)
			waiters = append(waiters, input)

			username[input.name] = true
			if len(username) >= 20 {
				mustRun = true
			}
		case <-ctx.Done():
			log.Println("tick")
			mustRun = true
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
		log.Println("execute", usernames)
		responseAll, err := execute(usernames)
		if err != nil {
			log.Println(err)

		}
		log.Println("done")

		log.Println(string(responseAll))

		username = map[string]bool{}

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

}
