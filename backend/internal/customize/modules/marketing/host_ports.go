package marketing

import "context"

// LotteryTransactions executes the callback once in one host transaction. An
// error from the callback or commit must roll back all its writes. Repositories,
// wallet and issuer receive the callback context without replacing it.
type LotteryTransactions interface {
	WithinLotteryTransaction(context.Context, func(context.Context) error) error
}

// LotteryWallet deliberately excludes account creation, permissions and unrelated
// user data. The host remains the authority for balance persistence/validation.
type LotteryWallet interface {
	GetBalance(context.Context, int64) (float64, error)
	UpdateBalance(context.Context, int64, float64) error
}

type BalanceRedeemInput struct {
	UserID int64
	Amount float64
	Notes  string
}

// T is the host's original redeem payload: neither its fields nor its JSON shape
// are copied into this module. The code is separately needed for the draw ledger.
type BalanceRedeemIssuer[T any] interface {
	IssueBalanceRedeem(context.Context, BalanceRedeemInput) (code string, payload *T, err error)
}
