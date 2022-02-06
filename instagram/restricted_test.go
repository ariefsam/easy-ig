package instagram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRestricted(t *testing.T) {
	str := `
    <h2>Restricted profile</h2>

    <p>
        You must be 18 years old or over to see this profile
    </p>

`
	isRestricted := IsRestricted(str)
	assert.Equal(t, isRestricted, "Must true")
}

func TestRestrictedIndonesia(t *testing.T) {
	str := `
    <h2>Profil dibatasi</h2>

    <p>
        You must be 18 years old or over to see this profile
    </p>

`
	isRestricted := IsRestricted(str)
	assert.Equal(t, isRestricted, "Must true")
}
