package label_model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Label struct {
	Id           string `json:"id" gorm:"type:uuid;primaryKey"`
	InstanceID   string `json:"instance_id" gorm:"type:uuid"`
	LabelID      string `json:"label_id"`
	LabelName    string `json:"label_name"`
	LabelColor   string `json:"label_color"`
	PredefinedId string `json:"predefined_id"`
}

func (m *Label) BeforeCreate(tx *gorm.DB) (err error) {
	m.Id = uuid.New().String()
	return
}
