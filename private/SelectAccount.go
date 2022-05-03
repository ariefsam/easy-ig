package private

import (
	"log"

	"github.com/alidhkh/gista"
)

var selection int

func SelectAccount() (ig *gista.Instagram, err error) {
	accounts := []struct {
		Username string
		Password string
	}{
		{
			"arief_hidayatulloh",
			"Indonesia1",
		},
		{
			"arieftoys",
			"Indonesia1",
		},
	}
	if len(accounts) == 0 {
		return
	}
	number := selection % len(accounts)
	log.Println(number)
	selection++

	ig, err = gista.New(nil)
	if err != nil {
		return
	}
	err = ig.Login(accounts[number].Username, accounts[number].Password, false)
	if err != nil {
		return
	}

	return
}
