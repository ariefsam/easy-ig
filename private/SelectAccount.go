package private

import (
	"log"
	"os"

	"github.com/ahmdrz/goinsta/v2"
	"github.com/tcnksm/go-input"
)

var selection int

func SelectAccount() (insta *goinsta.Instagram, err error) {
	accounts := []struct {
		Username string
		Password string
	}{
		// {
		// 	"ahmadzaani",
		// 	"Indonesia1",
		// },
		// {
		// 	"jastip_by_vininstore",
		// 	"Indonesia1",
		// },
		{
			"arieftoys",
			"Indonesia1",
		},
		{
			"arief_hidayatulloh",
			"Indonesia1",
		},
	}
	if len(accounts) == 0 {
		return
	}
	number := selection % len(accounts)
	log.Println(number)
	selection++

	path := "./sessions/" + accounts[number].Username

	insta, err = goinsta.Import(path)
	if err == nil {
		log.Println("no error")
		return
	} else {
		log.Println(err)
	}
	insta = goinsta.New(
		accounts[number].Username,
		accounts[number].Password,
	)
	if err = insta.Login(); err != nil {
		switch v := err.(type) {
		case goinsta.ChallengeError:
			err := insta.Challenge.Process(v.Challenge.APIPath)
			if err != nil {
				log.Fatalln(err)
			}

			ui := &input.UI{
				Writer: os.Stdout,
				Reader: os.Stdin,
			}

			query := "What is SMS code for instagram " + accounts[number].Username
			code, err := ui.Ask(query, &input.Options{
				Default:  "000000",
				Required: true,
				Loop:     true,
			})
			if err != nil {
				log.Fatalln(err)
			}

			err = insta.Challenge.SendSecurityCode(code)
			if err != nil {
				return nil, err
			}

			insta.Account = insta.Challenge.LoggedInUser
		default:
			log.Println("Login error:", err)
			return nil, err
		}
	}
	insta.Export(path)
	log.Printf("logged in as %s \n", insta.Account.Username)
	return
}

/*
if err := insta.Login(); err != nil {
		switch v := err.(type) {
		case goinsta.ChallengeError:
			err := insta.Challenge.Process(v.Challenge.APIPath)
			if err != nil {
				log.Fatalln(err)
			}

			ui := &input.UI{
				Writer: os.Stdout,
				Reader: os.Stdin,
			}

			query := "What is SMS code for instagram?"
			code, err := ui.Ask(query, &input.Options{
				Default:  "000000",
				Required: true,
				Loop:     true,
			})
			if err != nil {
				log.Fatalln(err)
			}

			err = insta.Challenge.SendSecurityCode(code)
			if err != nil {
				log.Fatalln(err)
			}

			insta.Account = insta.Challenge.LoggedInUser
		default:
			log.Fatalln(err)
		}

		log.Printf("logged in as %s \n", insta.Account.Username)
	}*/
