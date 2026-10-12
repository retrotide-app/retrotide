package board

import (
	"retrotide/internal/boardtemplate"
	"retrotide/internal/dto"
)

func (s *service) Templates() *dto.TemplateListResponse {
	catalogue := boardtemplate.All()

	templates := make([]dto.TemplateResponse, 0, len(catalogue))
	for _, template := range catalogue {
		templates = append(templates, templateResponse(template))
	}

	return &dto.TemplateListResponse{Templates: templates}
}

func (s *service) Template(key string) (*dto.TemplateResponse, error) {
	template, ok := boardtemplate.Lookup(key)
	if !ok {
		return nil, ErrTemplateNotFound
	}

	response := templateResponse(template)

	return &response, nil
}

func templateResponse(template boardtemplate.Template) dto.TemplateResponse {
	columns := make([]dto.TemplateColumnResponse, 0, len(template.Columns))
	for _, column := range template.Columns {
		columns = append(columns, dto.TemplateColumnResponse{Title: column.Title, Description: column.Description, Colour: column.Colour})
	}

	return dto.TemplateResponse{Key: template.Key, Name: template.Title, Description: template.Description, Columns: columns}
}
