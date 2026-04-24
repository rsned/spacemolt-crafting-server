// DEPRECATED: This command has been migrated to /home/robert/spacemolt/kb/
// See DEPRECATED.md for details. This code will be removed in a future version.
//
// Command generate-catalog reads the crafting database and produces one
// markdown file per item in the catalog output directory.
package main

import (
	"cmp"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	htmltpl "html/template"
	"text/template"

	humanize "github.com/dustin/go-humanize"
	_ "modernc.org/sqlite"
)

// Item holds every column from the items table plus its recipe relationships.
type Item struct {
	ID          string
	Name        string
	Description string
	Category    string
	Rarity      string
	Size        int
	BaseValue   int
	Stackable   bool
	Tradeable   bool

	ProducedBy []ProducedBy
	UsedIn     []UsedIn
}

// ProducedBy describes a recipe that produces this item.
type ProducedBy struct {
	RecipeID     string
	RecipeName   string
	Quantity     int
	CraftingTime int
	Skills       []SkillReq
}

// UsedIn describes a recipe that consumes this item and what it produces.
type UsedIn struct {
	RecipeID       string
	RecipeName     string
	Quantity       int
	OutputID       string
	OutputName     string
	OutputCategory string
}

// SkillReq pairs a skill name with its required level.
type SkillReq struct {
	Name  string
	Level int
}

func main() {
	dbPath := "database/crafting.db"
	outDir := "catalog"

	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	items, err := loadItems(db)
	if err != nil {
		log.Fatalf("load items: %v", err)
	}

	if err := loadProducedBy(db, items); err != nil {
		log.Fatalf("load produced-by: %v", err)
	}

	if err := loadUsedIn(db, items); err != nil {
		log.Fatalf("load used-in: %v", err)
	}

	// Clean generated markdown files so stale items don't linger,
	// but preserve non-generated content (e.g. catalog/images/).
	if err := cleanGeneratedFiles(outDir); err != nil {
		log.Fatalf("clean output dir: %v", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	tmpl := template.Must(template.New("item").Funcs(template.FuncMap{
		"yesno":      yesno,
		"fmtValue":   fmtValue,
		"joinSkills": joinSkills,
	}).Parse(itemTemplate))

	for _, item := range items {
		catDir := filepath.Join(outDir, item.Category)
		if err := os.MkdirAll(catDir, 0o755); err != nil {
			log.Fatalf("create category dir %s: %v", catDir, err)
		}
		path := filepath.Join(catDir, item.ID+".md")
		f, err := os.Create(path)
		if err != nil {
			log.Fatalf("create %s: %v", path, err)
		}
		if err := tmpl.Execute(f, item); err != nil {
			_ = f.Close()
			log.Fatalf("render %s: %v", item.ID, err)
		}
		if err := f.Close(); err != nil {
			log.Fatalf("close %s: %v", path, err)
		}
	}

	// Group items by category for READMEs.
	catItems := make(map[string][]*Item)
	for _, it := range items {
		catItems[it.Category] = append(catItems[it.Category], it)
	}
	for _, itemList := range catItems {
		slices.SortFunc(itemList, func(a, b *Item) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}

	categories := make([]CategoryInfo, 0, len(catItems))
	for cat, itemList := range catItems {
		categories = append(categories, CategoryInfo{
			Name:        cat,
			Description: categoryDescriptions[cat],
			Count:       len(itemList),
			Items:       itemList,
		})
	}
	slices.SortFunc(categories, func(a, b CategoryInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})

	if err := writeREADMEs(outDir, categories); err != nil {
		log.Fatalf("write READMEs: %v", err)
	}

	if err := writeHTMLPages(outDir, categories, items); err != nil {
		log.Fatalf("write HTML pages: %v", err)
	}

	fmt.Printf("Generated %d catalog pages in %s/\n", len(items), outDir)
}

// CategoryInfo groups items for README generation.
type CategoryInfo struct {
	Name        string
	Description string
	Count       int
	Items       []*Item
}

var categoryDescriptions = map[string]string{
	"artifact":   "Rare relics and ancient objects from lost civilizations.",
	"component":  "Crafted parts and assemblies used to build ships, stations, and equipment.",
	"consumable": "Single-use items including ammunition, stims, repair kits, and fuel.",
	"contraband": "Illegal goods that carry severe penalties if caught in possession.",
	"defense":    "Defensive equipment and shield systems.",
	"document":   "Blueprints, maps, and encrypted data files.",
	"drone":      "Autonomous craft for combat, mining, repair, and reconnaissance.",
	"material":   "Rare raw materials with special properties.",
	"misc":       "Collectibles, souvenirs, medals, and other miscellaneous items.",
	"ore":        "Raw ores, gases, ice, and biological samples harvested from space.",
	"refined":    "Processed materials refined from raw ores and gases.",
	"weapon":     "Weapons and weapon systems.",
}

func writeREADMEs(outDir string, categories []CategoryInfo) error {
	funcs := template.FuncMap{
		"yesno":      yesno,
		"fmtValue":   fmtValue,
		"joinSkills": joinSkills,
		"slug":       slug,
	}
	topTmpl := template.Must(template.New("top").Funcs(funcs).Parse(topREADMETemplate))
	catTmpl := template.Must(template.New("cat").Funcs(funcs).Parse(catREADMETemplate))

	// Top-level README.
	topPath := filepath.Join(outDir, "README.md")
	f, err := os.Create(topPath)
	if err != nil {
		return err
	}
	if err := topTmpl.Execute(f, categories); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Per-category READMEs.
	for _, cat := range categories {
		catPath := filepath.Join(outDir, cat.Name, "README.md")
		f, err := os.Create(catPath)
		if err != nil {
			return err
		}
		if err := catTmpl.Execute(f, cat); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func loadItems(db *sql.DB) (map[string]*Item, error) {
	rows, err := db.Query(`SELECT id, name, COALESCE(description,''), COALESCE(category,''), COALESCE(rarity,''), size, base_value, stackable, tradeable FROM items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make(map[string]*Item)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.Description, &it.Category, &it.Rarity, &it.Size, &it.BaseValue, &it.Stackable, &it.Tradeable); err != nil {
			return nil, err
		}
		items[it.ID] = &it
	}
	return items, rows.Err()
}

func loadProducedBy(db *sql.DB, items map[string]*Item) error {
	rows, err := db.Query(`
		SELECT ro.item_id, r.id, r.name, ro.quantity, r.crafting_time
		FROM recipe_outputs ro
		JOIN recipes r ON ro.recipe_id = r.id
		ORDER BY ro.item_id, r.id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	// Deduplicate by (item_id, recipe_id).
	type key struct{ itemID, recipeID string }
	seen := make(map[key]*ProducedBy)
	var order []key

	for rows.Next() {
		var itemID, recipeID, recipeName string
		var qty, craftTime int
		if err := rows.Scan(&itemID, &recipeID, &recipeName, &qty, &craftTime); err != nil {
			return err
		}
		k := key{itemID, recipeID}
		if _, ok := seen[k]; ok {
			continue
		}
		pb := &ProducedBy{
			RecipeID:     recipeID,
			RecipeName:   recipeName,
			Quantity:     qty,
			CraftingTime: craftTime,
		}
		seen[k] = pb
		order = append(order, k)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, k := range order {
		if it, ok := items[k.itemID]; ok {
			it.ProducedBy = append(it.ProducedBy, *seen[k])
		}
	}
	return nil
}

func loadUsedIn(db *sql.DB, items map[string]*Item) error {
	rows, err := db.Query(`
		SELECT ri.item_id, r.id, r.name, ri.quantity, ro.item_id, oi.name, COALESCE(oi.category, '')
		FROM recipe_inputs ri
		JOIN recipes r ON ri.recipe_id = r.id
		JOIN recipe_outputs ro ON r.id = ro.recipe_id
		JOIN items oi ON ro.item_id = oi.id
		ORDER BY ri.item_id, r.id, oi.name`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	// Deduplicate by (itemID, recipeID, outputID) since multi-output
	// recipes would otherwise produce duplicate rows.
	type key struct{ itemID, recipeID, outputID string }
	seen := make(map[key]struct{})

	for rows.Next() {
		var u UsedIn
		var itemID string
		if err := rows.Scan(&itemID, &u.RecipeID, &u.RecipeName, &u.Quantity, &u.OutputID, &u.OutputName, &u.OutputCategory); err != nil {
			return err
		}
		k := key{itemID, u.RecipeID, u.OutputID}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		if it, ok := items[itemID]; ok {
			it.UsedIn = append(it.UsedIn, u)
		}
	}
	return rows.Err()
}

func yesno(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func fmtValue(v int) string {
	return humanize.Comma(int64(v)) + " cr"
}

// cleanGeneratedFiles removes all .md files and category subdirectories from
// outDir while preserving non-generated content like the images/ directory.
func cleanGeneratedFiles(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(outDir, e.Name())
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".html")) {
			if err := os.Remove(path); err != nil {
				return err
			}
		} else if e.IsDir() && e.Name() != "images" {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// slug converts a name to a GitHub-compatible anchor: lowercase, spaces to hyphens.
func slug(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}

func joinSkills(skills []SkillReq) string {
	if len(skills) == 0 {
		return "None"
	}
	parts := make([]string, len(skills))
	for i, s := range skills {
		parts[i] = fmt.Sprintf("%s %d", s.Name, s.Level)
	}
	return strings.Join(parts, ", ")
}

const topREADMETemplate = `<!-- Auto-generated from crafting.db — do not edit manually -->

# SpaceMolt Item Catalog

Complete reference for all items in the SpaceMolt universe, organized by category.

| Category | Items | Description |
|----------|------:|-------------|
{{- range .}}
| [{{.Name}}]({{.Name}}/) | {{.Count}} | {{.Description}} |
{{- end}}
`

const catREADMETemplate = `<!-- Auto-generated from crafting.db — do not edit manually -->

# {{.Name}}

{{.Description}}

## Table of Contents

{{- range .Items}}
- [{{.Name}}](#{{slug .Name}})
{{- end}}

---
{{range .Items}}
## {{.Name}}

{{- /* Include the item file contents inline */}}

<table>
<tr><th colspan="2" style="text-align:center;"><h3>{{.Name}}</h3></th></tr>
<tr><td colspan="2" style="text-align:center;"><img src="../images/{{.ID}}.png" alt="{{.Name}}" width="128"></td></tr>
<tr><th colspan="2" style="text-align:center;">General</th></tr>
<tr><td><b>Rarity</b></td><td>{{.Rarity}}</td></tr>
<tr><td><b>Size</b></td><td>{{.Size}}</td></tr>
<tr><td><b>Stackable</b></td><td>{{yesno .Stackable}}</td></tr>
<tr><td><b>Tradeable</b></td><td>{{yesno .Tradeable}}</td></tr>
<tr><th colspan="2" style="text-align:center;">Market</th></tr>
<tr><td><b>Base Value</b></td><td>{{fmtValue .BaseValue}}</td></tr>
</table>

> {{.Description}}

[View full page]({{.ID}}.md)

---
{{end}}`

// rarityColor maps rarity levels to SMUI aurora palette hex colors.
func rarityColor(r string) string {
	switch strings.ToLower(r) {
	case "common":
		return "#a3be8c" // aurora green
	case "uncommon":
		return "#88c0d0" // frost-2
	case "rare":
		return "#5e81ac" // frost-4
	case "exotic":
		return "#d08770" // aurora orange
	case "legendary":
		return "#b48ead" // aurora purple
	default:
		return "#d8dee9" // snow storm
	}
}

func writeHTMLPages(outDir string, categories []CategoryInfo, items map[string]*Item) error {
	funcs := htmltpl.FuncMap{
		"yesno":       yesno,
		"fmtValue":    fmtValue,
		"joinSkills":  joinSkills,
		"rarityColor": rarityColor,
	}
	topTmpl := htmltpl.Must(htmltpl.New("top").Funcs(funcs).Parse(htmlTopTemplate))
	catTmpl := htmltpl.Must(htmltpl.New("cat").Funcs(funcs).Parse(htmlCatTemplate))
	itemHTMLTmpl := htmltpl.Must(htmltpl.New("item").Funcs(funcs).Parse(htmlItemTemplate))

	// Top-level index.html.
	topPath := filepath.Join(outDir, "index.html")
	f, err := os.Create(topPath)
	if err != nil {
		return err
	}
	if err := topTmpl.Execute(f, categories); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Per-category index.html.
	for _, cat := range categories {
		catPath := filepath.Join(outDir, cat.Name, "index.html")
		f, err := os.Create(catPath)
		if err != nil {
			return err
		}
		if err := catTmpl.Execute(f, cat); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}

	// Per-item HTML pages.
	for _, item := range items {
		path := filepath.Join(outDir, item.Category, item.ID+".html")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := itemHTMLTmpl.Execute(f, item); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

var sortScript = `<script>
document.querySelectorAll("table").forEach(function(table) {
  var headers = table.querySelectorAll("th");
  var sortCol = -1, sortAsc = true;
  headers.forEach(function(th, idx) {
    th.addEventListener("click", function() {
      if (sortCol === idx) { sortAsc = !sortAsc; } else { sortCol = idx; sortAsc = true; }
      headers.forEach(function(h) {
        var arrow = h.querySelector(".sort-arrow");
        if (arrow) arrow.remove();
      });
      var arrow = document.createElement("span");
      arrow.className = "sort-arrow";
      arrow.textContent = sortAsc ? "\u25B2" : "\u25BC";
      th.appendChild(arrow);
      var tbody = table.querySelector("tbody") || table;
      var rows = Array.from(tbody.querySelectorAll("tr")).filter(function(r) { return !r.querySelector("th"); });
      rows.sort(function(a, b) {
        var at = a.cells[idx].textContent.trim();
        var bt = b.cells[idx].textContent.trim();
        var an = at.replace(/[^0-9.-]/g, ""), bn = bt.replace(/[^0-9.-]/g, "");
        if (an !== "" && bn !== "" && !isNaN(an) && !isNaN(bn)) {
          return sortAsc ? an - bn : bn - an;
        }
        return sortAsc ? at.localeCompare(bt) : bt.localeCompare(at);
      });
      rows.forEach(function(r) { tbody.appendChild(r); });
    });
  });
});
</script>`

var htmlTopTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SpaceMolt Item Catalog</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
<style>
:root {
  --background: hsl(213, 16%, 12%);
  --foreground: hsl(213, 27%, 88%);
  --primary: hsl(193, 44%, 67%);
  --card: hsl(217, 16%, 15.5%);
  --card-header: hsl(217, 16%, 13%);
  --border: hsl(217, 17%, 28%);
  --muted-foreground: hsl(213, 12%, 55%);
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'JetBrains Mono', monospace;
  background: var(--background);
  color: var(--foreground);
  line-height: 1.6;
  padding: 2rem;
  max-width: 960px;
  margin: 0 auto;
}
a { color: var(--primary); text-decoration: none; }
a:hover { text-decoration: underline; }
h1 {
  font-size: 1.5rem;
  text-transform: uppercase;
  letter-spacing: 2px;
  margin-bottom: 1.5rem;
}
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 0;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  font-weight: 400;
}
td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
}
tr:last-child td { border-bottom: none; }
td.count { text-align: right; }
th { cursor: pointer; user-select: none; }
th:hover { color: var(--primary); }
th .sort-arrow { margin-left: 4px; font-size: 10px; }
</style>
</head>
<body>
<h1>SpaceMolt Item Catalog</h1>
<div class="card">
<table>
<tr><th>Category</th><th style="text-align:right">Items</th><th>Description</th></tr>
{{- range .}}
<tr>
  <td><a href="{{.Name}}/">{{.Name}}</a></td>
  <td class="count">{{.Count}}</td>
  <td>{{.Description}}</td>
</tr>
{{- end}}
</table>
</div>
` + sortScript + `
</body>
</html>
`

var htmlCatTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — SpaceMolt Item Catalog</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
<style>
:root {
  --background: hsl(213, 16%, 12%);
  --foreground: hsl(213, 27%, 88%);
  --primary: hsl(193, 44%, 67%);
  --card: hsl(217, 16%, 15.5%);
  --card-header: hsl(217, 16%, 13%);
  --border: hsl(217, 17%, 28%);
  --muted-foreground: hsl(213, 12%, 55%);
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'JetBrains Mono', monospace;
  background: var(--background);
  color: var(--foreground);
  line-height: 1.6;
  padding: 2rem;
  max-width: 85%;
  margin: 0 auto;
}
a { color: var(--primary); text-decoration: none; }
a:hover { text-decoration: underline; }
nav.breadcrumb {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  margin-bottom: 2rem;
}
nav.breadcrumb a { color: var(--muted-foreground); }
nav.breadcrumb a:hover { color: var(--primary); }
h1 {
  font-size: 1.5rem;
  text-transform: uppercase;
  letter-spacing: 2px;
  margin-bottom: 0.5rem;
}
.description {
  color: var(--muted-foreground);
  margin-bottom: 1.5rem;
}
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 0;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  font-weight: 400;
}
td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}
tr:last-child td { border-bottom: none; }
.rarity-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1px;
  border: 1px solid;
}
td.value { text-align: right; }
td.size { text-align: right; }
td.thumb img { height: 64px; image-rendering: pixelated; }
th { cursor: pointer; user-select: none; }
th:hover { color: var(--primary); }
th .sort-arrow { margin-left: 4px; font-size: 10px; }
</style>
</head>
<body>
<nav class="breadcrumb"><a href="../">Catalog</a> / {{.Name}}</nav>
<h1>{{.Name}}</h1>
<p class="description">{{.Description}}</p>
<div class="card">
<table>
<tr><th>Item</th><th></th><th>Rarity</th><th style="text-align:right">Size</th><th style="text-align:right">Base Value</th><th>Description</th></tr>
{{- range .Items}}
<tr>
  <td><a href="{{.ID}}.html">{{.Name}}</a></td>
  <td class="thumb"><img src="../images/{{.ID}}.png" alt="{{.Name}}"></td>
  <td><span class="rarity-badge" style="color:{{rarityColor .Rarity}};border-color:{{rarityColor .Rarity}}">{{.Rarity}}</span></td>
  <td class="size">{{.Size}}</td>
  <td class="value">{{fmtValue .BaseValue}}</td>
  <td>{{.Description}}</td>
</tr>
{{- end}}
</table>
</div>
` + sortScript + `
</body>
</html>
`

var htmlItemTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — SpaceMolt Item Catalog</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
<style>
:root {
  --background: hsl(213, 16%, 12%);
  --foreground: hsl(213, 27%, 88%);
  --primary: hsl(193, 44%, 67%);
  --card: hsl(217, 16%, 15.5%);
  --card-header: hsl(217, 16%, 13%);
  --border: hsl(217, 17%, 28%);
  --muted-foreground: hsl(213, 12%, 55%);
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'JetBrains Mono', monospace;
  background: var(--background);
  color: var(--foreground);
  line-height: 1.6;
  padding: 2rem;
  max-width: 960px;
  margin: 0 auto;
}
a { color: var(--primary); text-decoration: none; }
a:hover { text-decoration: underline; }
nav.breadcrumb {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  margin-bottom: 2rem;
}
nav.breadcrumb a { color: var(--muted-foreground); }
nav.breadcrumb a:hover { color: var(--primary); }
h1 {
  font-size: 1.5rem;
  text-transform: uppercase;
  letter-spacing: 2px;
  margin-bottom: 1.5rem;
}
.card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 0;
  margin-bottom: 1.5rem;
}
.item-image {
  text-align: center;
  padding: 1.5rem;
  border-bottom: 1px solid var(--border);
}
.item-image img { image-rendering: pixelated; }
.section-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  padding: 0.5rem 0.75rem;
  background: var(--card-header);
  border-bottom: 1px solid var(--border);
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  font-weight: 400;
}
td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
}
tr:last-child td { border-bottom: none; }
.kv-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  color: var(--muted-foreground);
  width: 140px;
}
.rarity-badge {
  display: inline-block;
  padding: 2px 8px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1px;
  border: 1px solid;
}
blockquote {
  border-left: 3px solid var(--border);
  padding: 0.75rem 1rem;
  color: var(--muted-foreground);
  font-style: italic;
  margin: 1rem 0;
}
</style>
</head>
<body>
<nav class="breadcrumb"><a href="../">Catalog</a> / <a href="./">{{.Category}}</a> / {{.Name}}</nav>
<h1>{{.Name}}</h1>
<div class="card">
  <div class="item-image">
    <img src="../images/{{.ID}}.png" alt="{{.Name}}" height="200">
  </div>
  <div class="section-label">General</div>
  <table>
    <tr><td class="kv-label">Category</td><td><a href="./">{{.Category}}</a></td></tr>
    <tr><td class="kv-label">Rarity</td><td><span class="rarity-badge" style="color:{{rarityColor .Rarity}};border-color:{{rarityColor .Rarity}}">{{.Rarity}}</span></td></tr>
    <tr><td class="kv-label">Size</td><td>{{.Size}}</td></tr>
    <tr><td class="kv-label">Stackable</td><td>{{yesno .Stackable}}</td></tr>
    <tr><td class="kv-label">Tradeable</td><td>{{yesno .Tradeable}}</td></tr>
  </table>
  <div class="section-label">Market</div>
  <table>
    <tr><td class="kv-label">Base Value</td><td>{{fmtValue .BaseValue}}</td></tr>
  </table>
</div>
<blockquote>{{.Description}}</blockquote>
{{- if or .ProducedBy .UsedIn}}
<div class="card">
{{- if .ProducedBy}}
  <div class="section-label">Produced By</div>
  <table>
    <tr><th>Recipe</th><th>Qty</th><th>Crafting Time</th><th>Skills</th></tr>
    {{- range .ProducedBy}}
    <tr><td>{{.RecipeName}}</td><td>{{.Quantity}}</td><td>{{.CraftingTime}} ticks</td><td>{{joinSkills .Skills}}</td></tr>
    {{- end}}
  </table>
{{- end}}
{{- if .UsedIn}}
  <div class="section-label">Used In</div>
  <table>
    <tr><th>Recipe</th><th>Qty</th><th>Produces</th></tr>
    {{- range .UsedIn}}
    <tr><td>{{.RecipeName}}</td><td>{{.Quantity}}</td><td><a href="../{{.OutputCategory}}/{{.OutputID}}.html">{{.OutputName}}</a></td></tr>
    {{- end}}
  </table>
{{- end}}
</div>
{{- end}}
</body>
</html>
`

const itemTemplate = `<!-- Auto-generated from crafting.db — do not edit manually -->

<table>
<tr><th colspan="2" style="text-align:center;"><h3>{{.Name}}</h3></th></tr>
<tr><td colspan="2" style="text-align:center;"><img src="../images/{{.ID}}.png" alt="{{.Name}}" width="128"></td></tr>
<tr><th colspan="2" style="text-align:center;">General</th></tr>
<tr><td><b>Category</b></td><td>{{.Category}}</td></tr>
<tr><td><b>Rarity</b></td><td>{{.Rarity}}</td></tr>
<tr><td><b>Size</b></td><td>{{.Size}}</td></tr>
<tr><td><b>Stackable</b></td><td>{{yesno .Stackable}}</td></tr>
<tr><td><b>Tradeable</b></td><td>{{yesno .Tradeable}}</td></tr>
<tr><th colspan="2" style="text-align:center;">Market</th></tr>
<tr><td><b>Base Value</b></td><td>{{fmtValue .BaseValue}}</td></tr>
</table>

> {{.Description}}
{{- if or .ProducedBy .UsedIn}}

## Crafting
{{- if .ProducedBy}}

### Produced By

| Recipe | Qty | Crafting Time | Skills Required |
|--------|-----|---------------|-----------------|
{{- range .ProducedBy}}
| {{.RecipeName}} | {{.Quantity}} | {{.CraftingTime}} ticks | {{joinSkills .Skills}} |
{{- end}}
{{- end}}
{{- if .UsedIn}}

### Used In

| Recipe | Qty | Produces |
|--------|-----|----------|
{{- range .UsedIn}}
| {{.RecipeName}} | {{.Quantity}} | [{{.OutputName}}](../{{.OutputCategory}}/{{.OutputID}}.md) |
{{- end}}
{{- end}}
{{- end}}
`
