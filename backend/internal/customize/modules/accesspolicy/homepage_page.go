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
	return RenderHomepageGateWithTurnstile(token, answer, returnPath, nonce, message, "", "")
}

func RenderHomepageGateWithTurnstile(token, answer, returnPath, nonce, message, siteKey, browserID string) ([]byte, error) {
	var image string
	var err error
	if siteKey == "" {
		image, err = HomepageCaptchaImage(answer)
		if err != nil {
			return nil, err
		}
	}
	var out bytes.Buffer
	err = homepageGateTemplate.Execute(&out, struct {
		Challenge, Return, Nonce, Error string
		SiteKey, BrowserID              string
		Image                           template.URL
	}{token, HomepageReturnPath(returnPath), nonce, message, siteKey, browserID, template.URL(image)}) // #nosec G203 -- locally generated PNG data URI only
	return out.Bytes(), err
}
