package branding

import _ "embed"

//go:embed banner.txt
var banner string

func Banner() string {
	return banner
}
