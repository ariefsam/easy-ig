package private_test

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/ariefhidayatulloh/easy-ig/ariefjson"
	"gitlab.com/ariefhidayatulloh/easy-ig/private"
)

func TestSelectAccount(t *testing.T) {
	for i := 0; i < 5; i++ {
		ig, err := private.SelectAccount()
		assert.NoError(t, err)
		assert.NotNil(t, ig.Account)
		cur, err := ig.Account.GetCurrentUser()
		assert.NoError(t, err)
		log.Println(ariefjson.Marshal(cur.User.Username))
		time.Sleep(1 * time.Second)
	}
}
