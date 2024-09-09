package types

type InterfaceLister interface {
	GetInterfaces(dbPath string) ([]string, error)
}
