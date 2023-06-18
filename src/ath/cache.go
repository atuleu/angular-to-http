package ath

type Create func() (any, error)

type Cache interface {
	Get(string, Create) (any, error)
}
