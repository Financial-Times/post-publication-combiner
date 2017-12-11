package processor

type MessageContent struct {
	ContentURI   string       `json:"contentUri"`
	ContentModel ContentModel `json:"payload"`
	LastModified string       `json:"lastModified"`
}

type ContentModel map[string]interface{}

type CombinedModel struct {
	UUID     string       `json:"uuid"`
	Content  ContentModel `json:"content"`
	Metadata []Annotation `json:"metadata"`

	ContentURI    string `json:"contentUri"`
	LastModified  string `json:"lastModified"`
	MarkedDeleted bool   `json:"markedDeleted"`
}

type Annotations struct {
	Annotations []Annotation `json:"annotations"`
	UUID        string       `json:"uuid"`
}

type Annotation struct {
	Thing `json:"thing,omitempty"`
}

type Thing struct {
	ID              string   `json:"id,omitempty"`
	PrefLabel       string   `json:"prefLabel,omitempty"`
	Types           []string `json:"types,omitempty"`
	Predicate       string   `json:"predicate,omitempty"`
	ApiUrl          string   `json:"apiUrl,omitempty"`
	LeiCode         string   `json:"leiCode,omitempty"`
	FactsetID       string   `json:"factsetID,omitempty"`
	TmeIDs          []string `json:"tmeIDs,omitempty"`
	UUIDs           []string `json:"uuids,omitempty"`
	PlatformVersion string   `json:"platformVersion,omitempty"`
}

//******************* GET EXPECTED VALUES *********************

func (cm ContentModel) getUUID() string {
	return cm["uuid"].(string)
}

func (cm ContentModel) getType() string {
	return cm["type"].(string)
}

func (cm ContentModel) getLastModified() string {
	return cm["lastModified"].(string)
}

func (cm ContentModel) getIdentifiers() []Identifier {
	return cm["identifiers"].([]Identifier)
}

type Identifier struct {
	Authority       string `json:"authority"`
	IdentifierValue string `json:"identifierValue"`
}

//******************* ADD VALUES *********************

func (cm *ContentModel) addUUID(uuid string) {
	(*cm)["uuid"] = uuid
}
