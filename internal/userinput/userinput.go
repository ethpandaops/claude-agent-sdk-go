// Package userinput provides typed structures for AskUserQuestion handling.
package userinput

import "context"

// QuestionOption represents a selectable choice in a user-input question.
type QuestionOption struct {
	Label       string
	Description string
}

// Question represents one AskUserQuestion prompt.
type Question struct {
	Question    string
	Header      string
	MultiSelect bool
	Options     []QuestionOption
}

// Answer contains selected or free-text responses for a single question.
type Answer struct {
	Answers []string
}

// Request contains parsed AskUserQuestion payload data.
type Request struct {
	Questions []Question
}

// Response contains answers keyed by question text.
type Response struct {
	Answers map[string]*Answer
}

// Callback handles AskUserQuestion requests and returns answers.
type Callback func(ctx context.Context, req *Request) (*Response, error)
