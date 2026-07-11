package extraction

import (
	"bytes"
	"encoding/json"
	"html"
	"mime"
	"net/url"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

type metadata struct {
	name, image, price, currency, canonical string
}

var whitespace = regexp.MustCompile(`\s+`)
var decimalPrice = regexp.MustCompile(`^\d+(?:\.\d{1,6})?$`)

func parseMetadata(body []byte, base *url.URL) (metadata, string) {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return metadata{}, ""
	}
	var jsonProducts []metadata
	fields := make(map[string]string)
	canonical := ""
	walkHTML(doc, func(node *xhtml.Node) {
		if node.Type != xhtml.ElementNode {
			return
		}
		switch strings.ToLower(node.Data) {
		case "script":
			if mediaType(attr(node, "type")) == "application/ld+json" {
				jsonProducts = append(jsonProducts, parseJSONLD(nodeText(node))...)
			}
		case "meta":
			key := strings.ToLower(strings.TrimSpace(firstNonEmpty(attr(node, "property"), attr(node, "name"), attr(node, "itemprop"))))
			value := normalizeText(attr(node, "content"))
			if fields[key] == "" {
				fields[key] = value
			}
		case "link":
			if containsToken(attr(node, "rel"), "canonical") && canonical == "" {
				canonical = strings.TrimSpace(attr(node, "href"))
			}
		}
	})
	fallback := metadata{
		name:      firstNonEmpty(fields["og:title"], fields["twitter:title"], fields["name"], normalizeText(findTitle(doc))),
		image:     firstNonEmpty(fields["og:image"], fields["twitter:image"], fields["image"]),
		price:     firstNonEmpty(fields["product:price:amount"], fields["og:price:amount"], fields["price"]),
		currency:  firstNonEmpty(fields["product:price:currency"], fields["og:price:currency"], fields["pricecurrency"]),
		canonical: canonical,
	}

	for _, candidate := range jsonProducts {
		merged := mergeMetadata(candidate, fallback)
		resolveMetadataURLs(&merged, base)
		if validMetadata(merged) {
			return merged, "json_ld"
		}
	}
	resolveMetadataURLs(&fallback, base)
	if validMetadata(fallback) {
		if fields["og:title"] != "" || fields["product:price:amount"] != "" || fields["og:price:amount"] != "" {
			return fallback, "open_graph"
		}
		return fallback, "html_metadata"
	}
	return metadata{}, ""
}

func parseJSONLD(raw string) []metadata {
	decoder := json.NewDecoder(strings.NewReader(html.UnescapeString(raw)))
	decoder.UseNumber()
	var root any
	if decoder.Decode(&root) != nil {
		return nil
	}
	var products []metadata
	findProducts(root, &products)
	return products
}

func findProducts(value any, products *[]metadata) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			findProducts(item, products)
		}
	case map[string]any:
		if hasType(typed["@type"], "Product") {
			*products = append(*products, productMetadata(typed))
		}
		if graph, ok := typed["@graph"]; ok {
			findProducts(graph, products)
		}
	}
}

func hasType(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), expected) || strings.HasSuffix(strings.ToLower(strings.TrimSpace(typed)), "/"+strings.ToLower(expected))
	case []any:
		for _, item := range typed {
			if hasType(item, expected) {
				return true
			}
		}
	}
	return false
}

func productMetadata(product map[string]any) metadata {
	result := metadata{
		name:  scalarString(product["name"]),
		image: imageString(product["image"]),
	}
	for _, offer := range offerObjects(product["offers"]) {
		price := firstNonEmpty(scalarString(offer["price"]), scalarString(offer["lowPrice"]))
		currency := scalarString(offer["priceCurrency"])
		if result.price == "" && price != "" {
			result.price = price
		}
		if result.currency == "" && currency != "" {
			result.currency = currency
		}
		if result.price != "" && result.currency != "" {
			break
		}
	}
	return result
}

func offerObjects(value any) []map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return []map[string]any{typed}
	case []any:
		var offers []map[string]any
		for _, item := range typed {
			if offer, ok := item.(map[string]any); ok {
				offers = append(offers, offer)
			}
		}
		return offers
	default:
		return nil
	}
}

func imageString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for _, item := range typed {
			if image := imageString(item); image != "" {
				return image
			}
		}
	case map[string]any:
		return firstNonEmpty(scalarString(typed["contentUrl"]), scalarString(typed["url"]))
	}
	return ""
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return normalizeText(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func validMetadata(value metadata) bool {
	value.name = normalizeText(value.name)
	value.price = strings.TrimSpace(value.price)
	value.currency = strings.ToUpper(strings.TrimSpace(value.currency))
	if value.name == "" || value.currency == "" || !decimalPrice.MatchString(value.price) {
		return false
	}
	for _, r := range value.currency {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	if len(value.currency) != 3 {
		return false
	}
	allZero := true
	for _, r := range value.price {
		if r >= '1' && r <= '9' {
			allZero = false
			break
		}
	}
	return !allZero
}

func mergeMetadata(primary, fallback metadata) metadata {
	primary.name = firstNonEmpty(primary.name, fallback.name)
	primary.image = firstNonEmpty(primary.image, fallback.image)
	primary.price = firstNonEmpty(primary.price, fallback.price)
	primary.currency = firstNonEmpty(primary.currency, fallback.currency)
	primary.canonical = firstNonEmpty(primary.canonical, fallback.canonical)
	return primary
}

func resolveMetadataURLs(value *metadata, base *url.URL) {
	value.name = normalizeText(value.name)
	value.price = strings.TrimSpace(value.price)
	value.currency = strings.ToUpper(strings.TrimSpace(value.currency))
	value.image = resolveReference(base, value.image)
	value.canonical = resolveReference(base, value.canonical)
}

func resolveReference(base *url.URL, raw string) string {
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return ""
	}
	resolved := base.ResolveReference(ref)
	if (resolved.Scheme != "http" && resolved.Scheme != "https") || resolved.User != nil || resolved.Hostname() == "" {
		return ""
	}
	return resolved.String()
}

func walkHTML(node *xhtml.Node, visit func(*xhtml.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func findTitle(node *xhtml.Node) string {
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "title") {
		return nodeText(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if title := findTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func nodeText(node *xhtml.Node) string {
	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func attr(node *xhtml.Node, key string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

func mediaType(raw string) string {
	value, _, err := mime.ParseMediaType(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(value)
}

func normalizeText(raw string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(html.UnescapeString(raw), " "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsToken(raw, token string) bool {
	for _, field := range strings.Fields(strings.ToLower(raw)) {
		if field == token {
			return true
		}
	}
	return false
}
