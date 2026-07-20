package entity

// @sk-task 20-shield-domain#T2.3: Implement Reaction type
//
// Reaction is a string type for domain values.
type Reaction string

const (
	ReactionAllow  Reaction = "allow"
	ReactionBlock  Reaction = "block"
	ReactionReview Reaction = "review"
	ReactionLog    Reaction = "log"
)

func (r Reaction) String() string { return string(r) }
