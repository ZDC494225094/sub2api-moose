package service

func ProvideUserBalanceSnapshotService(repo UserBalanceSnapshotRepository) *UserBalanceSnapshotService {
	svc := NewUserBalanceSnapshotService(repo)
	svc.Start()
	return svc
}
