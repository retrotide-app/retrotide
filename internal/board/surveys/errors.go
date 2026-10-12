package surveys

import (
	"errors"
)

var (
	ErrTitleRequired         = errors.New("a survey needs a title")
	ErrTitleTooLong          = errors.New("a survey title can be at most 120 characters")
	ErrQuestionsRequired     = errors.New("a survey needs at least one question")
	ErrTooManyQuestions      = errors.New("a survey can have at most 20 questions")
	ErrInvalidQuestionKind   = errors.New("a question must be single choice, multiple choice or a scale")
	ErrPromptRequired        = errors.New("every question needs a prompt")
	ErrPromptTooLong         = errors.New("a question prompt can be at most 300 characters")
	ErrInvalidOptions        = errors.New("a choice question needs 2 to 10 different options of at most 100 characters, and a scale takes none")
	ErrInvalidScale          = errors.New("a scale runs between two whole numbers from 0 to 10, lowest first, and a choice question takes none")
	ErrUnknownTemplate       = errors.New("there is no survey template with that key")
	ErrTemplateWithQuestions = errors.New("a survey comes from a template or from its own questions, never both")
	ErrAnswersRequired       = errors.New("answer at least one question")
	ErrInvalidAnswer         = errors.New("that answer does not fit its question")
	ErrUnknownQuestion       = errors.New("each answer must be to a different question of this survey")
	ErrTooManySurveys        = errors.New("a board can hold at most 20 surveys")
)
