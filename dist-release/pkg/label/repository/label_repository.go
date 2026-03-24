package label_repository

import (
	label_model "github.com/EvolutionAPI/evolution-go/pkg/label/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LabelRepository interface {
	InsertLabel(label label_model.Label) error
	UpdateLabel(label label_model.Label) error
	GetLabelByID(id string) (*label_model.Label, error)
	DeleteLabel(id string) error
	GetAllLabelsByInstanceID(instanceID string) ([]label_model.Label, error)
	UpsertLabel(label label_model.Label) error
}

type labelRepository struct {
	db *gorm.DB
}

func (l *labelRepository) InsertLabel(label label_model.Label) error {
	return l.db.Create(&label).Error
}

func (l *labelRepository) UpdateLabel(label label_model.Label) error {
	return l.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"label_id", "label_name", "label_color"}),
	}).Create(&label).Error
}

func (l *labelRepository) GetLabelByID(id string) (*label_model.Label, error) {
	var label label_model.Label
	err := l.db.Where("id = ?", id).First(&label).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &label, nil
}

func (l *labelRepository) DeleteLabel(id string) error {
	return l.db.Where("id = ?", id).Delete(&label_model.Label{}).Error
}

func (l *labelRepository) GetAllLabelsByInstanceID(instanceID string) ([]label_model.Label, error) {
	var labels []label_model.Label
	err := l.db.Where("instance_id = ?", instanceID).Find(&labels).Error
	if err != nil {
		return nil, err
	}
	return labels, nil
}

func (l *labelRepository) UpsertLabel(label label_model.Label) error {
	return l.db.Where("instance_id = ? AND label_id = ?",
		label.InstanceID,
		label.LabelID,
	).Assign(label_model.Label{
		LabelName:    label.LabelName,
		LabelColor:   label.LabelColor,
		PredefinedId: label.PredefinedId,
	}).FirstOrCreate(&label).Error
}

func NewLabelRepository(db *gorm.DB) LabelRepository {
	return &labelRepository{db: db}
}
