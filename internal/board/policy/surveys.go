package policy

import "retrotide/internal/model"

var (
	CanManageSurveys = CanFacilitate
)

func CanRespondToSurvey(access model.BoardAccess, survey model.Survey) error {
	if err := participating(access); err != nil {
		return err
	}

	if access.ParticipantID == nil {
		return ErrNotParticipant
	}

	if survey.Status != model.SurveyOpen {
		return ErrSurveyClosed
	}

	return nil
}
