package accesspolicy

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed homepage_gate.html
var homepageGateHTML string
var homepageGateTemplate = template.Must(template.New("homepage-gate").Parse(homepageGateHTML))

func RenderHomepageGate(token, answer, returnPath, nonce, message string) ([]byte, error) {
	image, err := HomepageCaptchaImage(answer)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = homepageGateTemplate.Execute(&out, struct {
		Challenge, Return, Nonce, Error string
		Image                           template.URL
	}{token, HomepageReturnPath(returnPath), nonce, message, template.URL(image)}) // #nosec G203 -- locally generated PNG data URI only
	return out.Bytes(), err
}
