package adapters

type Adapter interface {
	Upload(filePath string) (string, error)
}

var Adpaters map[string]Adapter = map[string]Adapter{
	"abyss": NewAbyssAdapter("lamine"),
}
