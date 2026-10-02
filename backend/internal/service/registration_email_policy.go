package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"

// Compatibility facade for settings, handlers and repository domain guards.
func RegistrationEmailSuffix(email string) string { return accesspolicy.RegistrationEmailSuffix(email) }
func RegistrationEmailDomain(email string) string { return accesspolicy.RegistrationEmailDomain(email) }
func NormalizeRegistrationEmailDomain(domain string) string {
	return accesspolicy.NormalizeRegistrationEmailDomain(domain)
}
func IsRegistrationEmailSuffixAllowed(email string, whitelist []string) bool {
	return accesspolicy.IsRegistrationEmailSuffixAllowed(email, whitelist)
}
func IsRegistrationEmailSuffixLimited(email string, whitelist []string) bool {
	return accesspolicy.IsRegistrationEmailSuffixLimited(email, whitelist)
}
func NormalizeRegistrationEmailSuffixWhitelist(raw []string) ([]string, error) {
	return accesspolicy.NormalizeRegistrationEmailSuffixWhitelist(raw)
}
func ParseRegistrationEmailSuffixWhitelist(raw string) []string {
	return accesspolicy.ParseRegistrationEmailSuffixWhitelist(raw)
}
