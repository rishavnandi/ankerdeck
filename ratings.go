package main

func ratingLegend() []Compat {
	var out []Compat
	for stars := 5; stars >= 0; stars-- {
		status, label := compatStatus(stars)
		out = append(out, Compat{Status: status, Label: label, Stars: stars})
	}
	return append(out, Compat{Status: "none", Label: "No report"})
}

func compatStatus(stars int) (string, string) {
	switch stars {
	case 5:
		return "great", "Runs great"
	case 4:
		return "good", "Runs well"
	case 3:
		return "playable", "Some issues"
	case 2:
		return "issues", "Major issues"
	case 1:
		return "broken", "Broken"
	default:
		return "untested", "Untested"
	}
}
