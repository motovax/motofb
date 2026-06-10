package parser

import (
	"strconv"
	"strings"

	"github.com/motovax/motofb/models"
)

func extractLatLon(uri string) (lat, lon *float64) {
	markerPos := strings.LastIndex(uri, "%7C")
	if markerPos == -1 {
		return nil, nil
	}
	coordsStr := uri[markerPos+3:]
	if ampPos := strings.Index(coordsStr, "&"); ampPos != -1 {
		coordsStr = coordsStr[:ampPos]
	}
	coordsStr = strings.ReplaceAll(coordsStr, "%2C", ",")
	parts := strings.SplitN(coordsStr, ",", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	latF, err1 := strconv.ParseFloat(parts[0], 64)
	lonF, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	return &latF, &lonF
}

func (p *Parser) parsePostExtensible(data map[string]any) models.PostAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	target, _ := story["target"].(map[string]any)
	_ = target
	return models.PostAttachment{
		ID:          strVal(data["legacy_attachment_id"]),
		Title:       strVal(story["title"]),
		Description: strVal(story["description"]),
		PostURL:     strVal(story["url"]),
	}
}

func (p *Parser) parseStoryExtensible(data map[string]any) models.SharedAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	return models.SharedAttachment{
		ID:          strVal(data["legacy_attachment_id"]),
		Title:       strVal(story["title"]),
		Description: strVal(story["description"]),
	}
}

func (p *Parser) parseReelExtensible(data map[string]any) models.ReelAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	target, _ := story["target"].(map[string]any)
	return models.ReelAttachment{
		ID:          strVal(data["legacy_attachment_id"]),
		URL:         strVal(story["url"]),
		VideoID:     strVal(target["video_id"]),
		Title:       strVal(story["title"]),
		Description: strVal(story["description"]),
	}
}

func (p *Parser) parseProfileExtensible(data map[string]any) models.ProfileAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	target, _ := story["target"].(map[string]any)
	pic := ""
	if cover, ok := target["cover_photo"].(map[string]any); ok {
		if photo, ok := cover["photo"].(map[string]any); ok {
			if image, ok := photo["image"].(map[string]any); ok {
				pic = strVal(image["uri"])
			}
		}
	}
	return models.ProfileAttachment{
		ID:             strVal(data["legacy_attachment_id"]),
		ProfileID:      strVal(target["id"]),
		ProfileName:    strVal(target["name"]),
		ProfileURL:     strVal(story["url"]),
		ProfilePicture: strVal(target["picture"]),
		CoverPhoto:     pic,
	}
}

func (p *Parser) parseExternalExtensible(data map[string]any) models.ExternalAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	return models.ExternalAttachment{
		ID:          strVal(data["legacy_attachment_id"]),
		URL:         strVal(story["url"]),
		Title:       strVal(story["title"]),
		Description: strVal(story["description"]),
	}
}

func (p *Parser) parseLocationExtensible(data map[string]any) models.LocationAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	media, _ := story["media"].(map[string]any)
	preview, _ := media["preview"].(map[string]any)
	url := strVal(preview["url"])
	lat, lon := extractLatLon(url)
	att := models.LocationAttachment{
		ID:      strVal(data["legacy_attachment_id"]),
		URL:     strVal(story["url"]),
		Address: strVal(story["description"]),
	}
	if lat != nil && lon != nil {
		att.Latitude = *lat
		att.Longitude = *lon
	}
	return att
}

func (p *Parser) parseProductExtensible(data map[string]any) models.ProductAttachment {
	story, _ := data["story_attachment"].(map[string]any)
	return models.ProductAttachment{
		ID:           strVal(data["legacy_attachment_id"]),
		ProductName:  strVal(story["title"]),
		ProductPrice: strVal(story["description"]),
		URL:          strVal(story["url"]),
	}
}