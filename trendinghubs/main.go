package trendinghubs

import (
	"appengine"
	"appengine/urlfetch"
	// "fmt"
	"html"
	"http"
	"os"
	"strings"
	"template"
)

func init() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/feed", generateFeed)
}

func generateFeed(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	list, e := GetTrendingRepositories(client)
	if e != nil {
		http.Error(w, e.String(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/rss+xml")
	tpl := template.Must(template.New("feed").Parse(rawtemplate))
	tpl.Execute(w, list)
}

var (
	rawtemplate = `
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>GitHub - Trending GitHub Repositories</title>
    <link>https://www.github.com</link>
    <description>The daily trending repositories</description>
    <language>en-US</language>
    {{range .}}
    <item>
      <title>{{.User}} / {{.Name}}</title>
      <link>https://www.github.com/{{.User}}/{{.Name}}</link>
      <author>{{.User}}</author>
    </item>
	{{end}}
  </channel>
</rss>
	`
)

type Repository struct {
	User string
	Name string
}

func GetTrendingRepositories(client *http.Client) ([]Repository, os.Error) {
	resp, e := client.Get("https://www.github.com/explore")
	if e != nil {
		return nil, e
	}
	rootnode, e := html.Parse(resp.Body)
	if e != nil {
		return nil, e
	}

	list := make([]Repository, 0, 5)
	listnode := findNextNodeWithClass(rootnode, "ranked-repositories")
	for _, item := range listnode.Child {
		repo := findNextNodeWithTag(item, "h3")
		if repo != nil && !hasClass("last", item) {
			owner := repo.Child[1].Child[0].Data
			name := repo.Child[3].Child[0].Data
			list = append(list, Repository{
				User: owner,
				Name: name,
			})
		}
	}
	return list, nil

}

type QualFunc func(node *html.Node) bool

func findNextNode(start *html.Node, hasQualifier QualFunc) *html.Node {
	if hasQualifier(start) {
		return start
	}
	for _, child := range start.Child {
		subnode := findNextNode(child, hasQualifier)
		if subnode != nil {
			return subnode
		}
	}
	return nil
}

func findNextNodeWithClass(start *html.Node, classname string) *html.Node {
	return findNextNode(start, func(node *html.Node) bool {
		return hasClass(classname, node)
	})
}

func findNextNodeWithTag(start *html.Node, tagname string) *html.Node {
	return findNextNode(start, func(node *html.Node) bool {
		return node.Data == tagname
	})
}

func hasClass(classname string, node *html.Node) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			classes := strings.Fields(attr.Val)
			for _, class := range classes {
				if class == classname {
					return true
				}
			}
		}
	}
	return false
}
