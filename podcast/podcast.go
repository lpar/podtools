package podcast

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type RSS struct {
	AttrXmlnsItunes string   `xml:"xmlns itunes,attr"`
	AttrVersion     string   `xml:"version,attr"`
	Channel         *Channel `xml:"channel,omitempty"`
}

type Image struct {
	AttrHref string   `xml:"href,attr"`
	XMLName  xml.Name `xml:"image,omitempty"`
}

type Category struct {
	AttrText string   `xml:"text,attr"`
	XMLName  xml.Name `xml:"category,omitempty"`
}

type Channel struct {
	Author      string      `xml:"author,omitempty"`
	Category    []*Category `xml:"category,omitempty"`
	Copyright   string      `xml:"copyright,omitempty"`
	Description string      `xml:"description,omitempty"`
	Explicit    string      `xml:"explicit,omitempty"`
	Image       *Image      `xml:"image,omitempty"`
	Item        []*Item     `xml:"item,omitempty"`
	Language    string      `xml:"language,omitempty"`
	LastBuild   *Timestamp  `xml:"lastBuildDate,omitempty"`
	Link        string      `xml:"link,omitempty"`
	Owner       *Owner      `xml:"owner,omitempty"`
	PubString   string      `xml:"pubDate,omitempty"` // TODO: Parse
	Subtitle    string      `xml:"subtitle,omitempty"`
	Summary     string      `xml:"summary,omitempty"`
	Title       string      `xml:"title,omitempty"`
}

type Enclosure struct {
	Length   int    `xml:"length,attr"`
	MIMEType string `xml:"type,attr"`
	URL      string `xml:"url,attr"`
}

type Guid struct {
	AttrIsPermaLink string `xml:"isPermaLink,attr"`
	Text            string `xml:",chardata"`
}

type Item struct {
	Author      string     `xml:"author,omitempty"`
	Category    string     `xml:"category,omitempty"`
	Description string     `xml:"description,omitempty"`
	Duration    Duration   `xml:"duration,omitempty"`
	Enclosure   *Enclosure `xml:"enclosure,omitempty"`
	Guid        *Guid      `xml:"guid,omitempty"`
	Keywords    Keywords   `xml:"keywords,omitempty"` // TODO: Parse
	PubDate     Timestamp  `xml:"pubDate,omitempty"`
	Title       string     `xml:"title,omitempty"`
}

type Owner struct {
	Email   string   `xml:"email,omitempty"`
	Name    string   `xml:"name,omitempty"`
	XMLName xml.Name `xml:"owner,omitempty"`
}

// Keyword unmarshaling

type Keywords []string

func (kw *Keywords) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var content string
	err := dec.DecodeElement(&content, &start)
	if err != nil {
		return err
	}
	keys := strings.Split(content, ",")
	for i, k := range keys {
		keys[i] = strings.Trim(k, " \n\t")
	}
	*kw = keys
	return err
}

// Custom Timestamp unmarshaling

type Timestamp struct {
	time.Time
}

func (ts *Timestamp) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var content string
	err := dec.DecodeElement(&content, &start)
	if err != nil {
		return err
	}
	t, err := time.Parse(time.RFC1123Z, content)
	if err == nil {
		*ts = Timestamp{t}
	}
	return err
}

// Custom Duration unmarshaling

type Duration time.Duration

func (dur *Duration) String() string {
	return time.Duration(*dur).String()
}

var babylon = []int{1, 60, 3600, 86400}

func parseDuration(ds string) (time.Duration, error) {
	chunks := strings.Split(ds, ":")
	lc := len(chunks)
	secs := 0
	for i := 0; i < lc; i++ {
		j := lc - i - 1
		c := chunks[j]
		s, err := strconv.Atoi(c)
		if err != nil {
			return time.Duration(0), fmt.Errorf("can't parse %s as duration, %s not integer: %s", ds, c, err)
		}
		secs += s * babylon[i]
	}
	return time.Duration(secs) * time.Second, nil
}

func (dur *Duration) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var content string
	err := dec.DecodeElement(&content, &start)
	if err != nil {
		return err
	}
	d, err := parseDuration(content)
	if err == nil {
		*dur = Duration(d)
	}
	return err
}
