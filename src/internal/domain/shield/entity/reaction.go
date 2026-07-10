package entity

// @sk-task 20-shield-domain#T2.3: Implement Reaction type
type Reaction string

const (
	ReactionAllow  Reaction = "allow"
	ReactionBlock  Reaction = "block"
	ReactionReview Reaction = "review"
	ReactionLog    Reaction = "log"
)

func (r Reaction) String() string { return string(r) }
