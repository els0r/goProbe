package types

type InterfaceLister interface {
	GetInterfaces() ([]string, error)
}
