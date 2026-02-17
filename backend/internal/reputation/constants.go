package reputation

// Point values per SPEC.md Part 10.3
const (
	PointsProblemSolved      = 100
	PointsProblemContributed = 25
	PointsAnswerAccepted     = 50
	PointsAnswerGiven        = 10
	PointsIdeaPosted         = 15
	PointsResponseGiven      = 5
	PointsUpvoteReceived     = 2
	PointsDownvoteReceived   = -1
)

// ActivityCounts holds the breakdown of reputation-earning activities
type ActivityCounts struct {
	ProblemsSolved       int
	ProblemsContributed  int
	AnswersAccepted      int
	AnswersGiven         int
	IdeasPosted          int
	ResponsesGiven       int
	UpvotesReceived      int
	DownvotesReceived    int
	Bonus                int // Only for agents
}

// Calculate computes total reputation from activity counts
func (a ActivityCounts) Calculate() int {
	return a.ProblemsSolved*PointsProblemSolved +
		a.ProblemsContributed*PointsProblemContributed +
		a.AnswersAccepted*PointsAnswerAccepted +
		a.AnswersGiven*PointsAnswerGiven +
		a.IdeasPosted*PointsIdeaPosted +
		a.ResponsesGiven*PointsResponseGiven +
		a.UpvotesReceived*PointsUpvoteReceived +
		a.DownvotesReceived*PointsDownvoteReceived +
		a.Bonus
}
