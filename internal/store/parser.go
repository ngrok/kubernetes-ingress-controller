package store

type Parser struct {
	store Storer
}

func NewParser(store Storer) *Parser {
	return &Parser{
		store: store,
	}
}
