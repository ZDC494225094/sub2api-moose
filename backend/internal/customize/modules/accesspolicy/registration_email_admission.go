package accesspolicy

// EmailAdmission describes policy, not a database authorization. A nonempty
// LimitedDomain still requires the host's atomic alias+domain guard on INSERT.
type EmailAdmission struct {
	Allowed       bool
	LimitedDomain string
}

func EvaluateRegistrationEmail(email string, whitelist []string, domainQuotaEnabled bool) EmailAdmission {
	if !IsRegistrationEmailSuffixLimited(email, whitelist) {
		return EmailAdmission{Allowed: true}
	}
	if !domainQuotaEnabled {
		return EmailAdmission{}
	}
	domain := RegistrationEmailDomain(email)
	if domain == "" {
		return EmailAdmission{}
	}
	return EmailAdmission{Allowed: true, LimitedDomain: domain}
}
