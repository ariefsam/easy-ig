package instagram

import "strings"

func IsRestricted(str string) (b bool) {
	b = strings.Contains(str, "Restricted profile")
	if !b {
		b = strings.Contains(str, "Profil dibatasi")
	}
	return
}
