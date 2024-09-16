package types

type InterfaceLister interface {
	ListInterfaces() ([]string, error)
}
