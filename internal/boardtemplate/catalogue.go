package boardtemplate

import (
	"slices"

	"retrotide/internal/model"
)

type (
	Column struct {
		Title       string
		Description string
		Colour      model.Colour
	}

	Template struct {
		Key         string
		Title       string
		Description string
		Columns     []Column
	}
)

const (
	BlankKey = "blank"
)

var (
	catalogue = []Template{
		{
			Key:         "start-stop-continue",
			Title:       "Start, Stop, Continue",
			Description: "Decide what the team should begin, give up and keep doing.",
			Columns: []Column{
				{Title: "Start", Description: "What should we begin doing?", Colour: model.ColourGreen},
				{Title: "Stop", Description: "What should we stop doing?", Colour: model.ColourRed},
				{Title: "Continue", Description: "What is working that we should keep doing?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "mad-sad-glad",
			Title:       "Mad, Sad, Glad",
			Description: "Share how the sprint felt, from frustrations to wins.",
			Columns: []Column{
				{Title: "Mad", Description: "What frustrated you?", Colour: model.ColourRed},
				{Title: "Sad", Description: "What disappointed you?", Colour: model.ColourBlue},
				{Title: "Glad", Description: "What made you happy?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "went-well",
			Title:       "Went Well, To Improve, Ideas",
			Description: "The classic retro: celebrate, fix and suggest.",
			Columns: []Column{
				{Title: "Went well", Description: "What went well this sprint?", Colour: model.ColourGreen},
				{Title: "To improve", Description: "What could have gone better?", Colour: model.ColourAmber},
				{Title: "Ideas", Description: "What should we try next?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "four-ls",
			Title:       "4Ls",
			Description: "Look back at what the team liked, learned, lacked and longed for.",
			Columns: []Column{
				{Title: "Liked", Description: "What did you enjoy?", Colour: model.ColourGreen},
				{Title: "Learned", Description: "What did you learn?", Colour: model.ColourBlue},
				{Title: "Lacked", Description: "What was missing?", Colour: model.ColourRed},
				{Title: "Longed for", Description: "What do you wish we had?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "sailboat",
			Title:       "Sailboat",
			Description: "Picture the team as a boat: what drives it, what drags it and what lies ahead.",
			Columns: []Column{
				{Title: "Wind", Description: "What pushed us forward?", Colour: model.ColourGreen},
				{Title: "Anchors", Description: "What held us back?", Colour: model.ColourRed},
				{Title: "Rocks", Description: "What risks lie ahead?", Colour: model.ColourAmber},
				{Title: "Island", Description: "Where are we heading?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "starfish",
			Title:       "Starfish",
			Description: "Tune the team's habits across five arms, from more of to stop.",
			Columns: []Column{
				{Title: "Keep doing", Description: "What works and should stay as it is?", Colour: model.ColourGreen},
				{Title: "More of", Description: "What should we do more of?", Colour: model.ColourTeal},
				{Title: "Less of", Description: "What should we do less of?", Colour: model.ColourAmber},
				{Title: "Stop doing", Description: "What should we give up?", Colour: model.ColourRed},
				{Title: "Start doing", Description: "What should we try?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "kalm",
			Title:       "KALM",
			Description: "Keep, add, more and less: adjust the team's practices.",
			Columns: []Column{
				{Title: "Keep", Description: "What should we keep doing?", Colour: model.ColourGreen},
				{Title: "Add", Description: "What should we add?", Colour: model.ColourBlue},
				{Title: "More", Description: "What should we do more of?", Colour: model.ColourTeal},
				{Title: "Less", Description: "What should we do less of?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "daki",
			Title:       "DAKI",
			Description: "Drop, add, keep and improve: decide what changes next sprint.",
			Columns: []Column{
				{Title: "Drop", Description: "What should we drop?", Colour: model.ColourRed},
				{Title: "Add", Description: "What should we add?", Colour: model.ColourBlue},
				{Title: "Keep", Description: "What should we keep?", Colour: model.ColourGreen},
				{Title: "Improve", Description: "What should we improve?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "rose-thorn-bud",
			Title:       "Rose, Thorn, Bud",
			Description: "Highlights, pain points and the ideas just starting to grow.",
			Columns: []Column{
				{Title: "Rose", Description: "What went well?", Colour: model.ColourPink},
				{Title: "Thorn", Description: "What was painful?", Colour: model.ColourRed},
				{Title: "Bud", Description: "What new ideas are budding?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "wrap",
			Title:       "WRAP",
			Description: "Wishes, risks, appreciations and puzzles: a rounded look back.",
			Columns: []Column{
				{Title: "Wishes", Description: "What do you wish for?", Colour: model.ColourPurple},
				{Title: "Risks", Description: "What risks do you see?", Colour: model.ColourRed},
				{Title: "Appreciations", Description: "Who or what do you appreciate?", Colour: model.ColourPink},
				{Title: "Puzzles", Description: "What still puzzles you?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "lean-coffee",
			Title:       "Lean Coffee",
			Description: "Gather topics, then talk them through in order.",
			Columns: []Column{
				{Title: "To discuss", Description: "What would you like to talk about?", Colour: model.ColourGrey},
				{Title: "Discussing", Description: "What are we talking about now?", Colour: model.ColourAmber},
				{Title: "Discussed", Description: "What have we covered?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "three-little-pigs",
			Title:       "Three Little Pigs",
			Description: "Test how sturdy the team's work is, from straw to bricks.",
			Columns: []Column{
				{Title: "House of straw", Description: "What is fragile and could fall over?", Colour: model.ColourAmber},
				{Title: "House of sticks", Description: "What stands but needs shoring up?", Colour: model.ColourTeal},
				{Title: "House of bricks", Description: "What is solid?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "hot-air-balloon",
			Title:       "Hot Air Balloon",
			Description: "Picture the team as a balloon: what lifts it, what weighs it down and the weather ahead.",
			Columns: []Column{
				{Title: "Hot air", Description: "What lifted us up?", Colour: model.ColourGreen},
				{Title: "Sandbags", Description: "What weighed us down?", Colour: model.ColourRed},
				{Title: "Storms", Description: "What risks could blow us off course?", Colour: model.ColourAmber},
				{Title: "Sunny skies", Description: "What good things lie ahead?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "speed-car",
			Title:       "Speed Car",
			Description: "Picture the team as a racing car: what speeds it up, what slows it and what lies on the road.",
			Columns: []Column{
				{Title: "Engine", Description: "What drove us forward?", Colour: model.ColourGreen},
				{Title: "Parachute", Description: "What slowed us down?", Colour: model.ColourRed},
				{Title: "Abyss", Description: "What dangers lie ahead?", Colour: model.ColourAmber},
				{Title: "Bridge", Description: "What could carry us over them?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "mountain-climb",
			Title:       "Mountain Climb",
			Description: "Chart the climb from base camp to the summit, and what helps or hurts on the way.",
			Columns: []Column{
				{Title: "Summit", Description: "What goal are we climbing towards?", Colour: model.ColourBlue},
				{Title: "Ropes", Description: "What kept us safe and moving up?", Colour: model.ColourGreen},
				{Title: "Rockfall", Description: "What knocked us back?", Colour: model.ColourRed},
				{Title: "Base camp", Description: "Where did we start from?", Colour: model.ColourGrey},
			},
		},
		{
			Key:         "desert-island",
			Title:       "Desert Island",
			Description: "Decide what the team would take to a desert island, what it would leave and how it would call for help.",
			Columns: []Column{
				{Title: "Bring along", Description: "What would you take with you?", Colour: model.ColourGreen},
				{Title: "Leave behind", Description: "What would you happily leave on the shore?", Colour: model.ColourRed},
				{Title: "Rescue signals", Description: "Where do we need help?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "weather-report",
			Title:       "Weather Report",
			Description: "Describe the sprint as a forecast, from bright spells to storms.",
			Columns: []Column{
				{Title: "Sunshine", Description: "What went brilliantly?", Colour: model.ColourAmber},
				{Title: "Clouds", Description: "What dulled the sprint?", Colour: model.ColourGrey},
				{Title: "Storms", Description: "What caused real trouble?", Colour: model.ColourRed},
				{Title: "Rainbows", Description: "What good came out of the bad weather?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "traffic-lights",
			Title:       "Traffic Lights",
			Description: "Sort the team's work into go, caution and stop.",
			Columns: []Column{
				{Title: "Green light", Description: "What should carry on as it is?", Colour: model.ColourGreen},
				{Title: "Amber light", Description: "What needs watching?", Colour: model.ColourAmber},
				{Title: "Red light", Description: "What should stop?", Colour: model.ColourRed},
			},
		},
		{
			Key:         "plus-delta",
			Title:       "Plus, Delta",
			Description: "A quick look at what worked and what to change.",
			Columns: []Column{
				{Title: "Plus", Description: "What worked well?", Colour: model.ColourGreen},
				{Title: "Delta", Description: "What would you change?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "keep-drop-add",
			Title:       "Keep, Drop, Add",
			Description: "Decide what the team keeps, drops and adds next sprint.",
			Columns: []Column{
				{Title: "Keep", Description: "What should we keep doing?", Colour: model.ColourGreen},
				{Title: "Drop", Description: "What should we drop?", Colour: model.ColourRed},
				{Title: "Add", Description: "What should we add?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "good-bad-ugly",
			Title:       "The Good, the Bad and the Ugly",
			Description: "An honest look at the best, the worst and the messiest parts of the sprint.",
			Columns: []Column{
				{Title: "The good", Description: "What went well?", Colour: model.ColourGreen},
				{Title: "The bad", Description: "What went badly?", Colour: model.ColourAmber},
				{Title: "The ugly", Description: "What got really messy?", Colour: model.ColourRed},
			},
		},
		{
			Key:         "fun-done-learn",
			Title:       "Fun, Done, Learn",
			Description: "Look back at what the team enjoyed, delivered and learned.",
			Columns: []Column{
				{Title: "Fun", Description: "What did you enjoy?", Colour: model.ColourPink},
				{Title: "Done", Description: "What did we deliver?", Colour: model.ColourGreen},
				{Title: "Learn", Description: "What did you learn?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "highs-lows",
			Title:       "Highs and Lows",
			Description: "Share the best and the worst moments of the sprint.",
			Columns: []Column{
				{Title: "Highs", Description: "What was the best part?", Colour: model.ColourGreen},
				{Title: "Lows", Description: "What was the worst part?", Colour: model.ColourRed},
			},
		},
		{
			Key:         "timeline",
			Title:       "Timeline",
			Description: "Walk through the sprint from start to finish.",
			Columns: []Column{
				{Title: "Beginning", Description: "What happened early on?", Colour: model.ColourBlue},
				{Title: "Middle", Description: "What happened midway through?", Colour: model.ColourTeal},
				{Title: "End", Description: "What happened towards the end?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "learning-matrix",
			Title:       "Learning Matrix",
			Description: "Four quick quadrants: what pleased you, what did not, ideas and thanks.",
			Columns: []Column{
				{Title: "Happy", Description: "What made you happy?", Colour: model.ColourGreen},
				{Title: "Unhappy", Description: "What made you unhappy?", Colour: model.ColourRed},
				{Title: "Ideas", Description: "What ideas do you have?", Colour: model.ColourPurple},
				{Title: "Appreciations", Description: "Who or what do you appreciate?", Colour: model.ColourPink},
			},
		},
		{
			Key:         "what-so-what-now-what",
			Title:       "What, So What, Now What",
			Description: "Move from the facts to what they mean and then to action.",
			Columns: []Column{
				{Title: "What", Description: "What happened?", Colour: model.ColourBlue},
				{Title: "So what", Description: "Why does it matter?", Colour: model.ColourAmber},
				{Title: "Now what", Description: "What do we do next?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "lessons-learned",
			Title:       "Lessons Learned",
			Description: "Capture what worked, what did not and what the team would do differently.",
			Columns: []Column{
				{Title: "What worked", Description: "What should we repeat?", Colour: model.ColourGreen},
				{Title: "What did not", Description: "What fell short?", Colour: model.ColourRed},
				{Title: "What we would change", Description: "What would we do differently next time?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "post-mortem",
			Title:       "Project Post-mortem",
			Description: "Look back on a finished project, from what happened to how to prevent it next time.",
			Columns: []Column{
				{Title: "What happened", Description: "What were the key events?", Colour: model.ColourGrey},
				{Title: "Impact", Description: "What did it affect?", Colour: model.ColourRed},
				{Title: "Root causes", Description: "Why did it happen?", Colour: model.ColourAmber},
				{Title: "Prevention", Description: "How do we stop it happening again?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "incident-review",
			Title:       "Incident Review",
			Description: "Review an incident without blame and agree the follow-ups.",
			Columns: []Column{
				{Title: "Timeline", Description: "What happened, and when?", Colour: model.ColourGrey},
				{Title: "What went well", Description: "What helped during the response?", Colour: model.ColourGreen},
				{Title: "What went wrong", Description: "What made the incident worse?", Colour: model.ColourRed},
				{Title: "Follow-ups", Description: "What actions will we take?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "pre-mortem",
			Title:       "Pre-mortem",
			Description: "Imagine the project has failed, then work out how to stop that happening.",
			Columns: []Column{
				{Title: "What could go wrong", Description: "What might make this fail?", Colour: model.ColourRed},
				{Title: "Warning signs", Description: "How would we know it is going wrong?", Colour: model.ColourAmber},
				{Title: "How we prevent it", Description: "What can we do now to avoid it?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "swot",
			Title:       "SWOT",
			Description: "Weigh up strengths, weaknesses, opportunities and threats.",
			Columns: []Column{
				{Title: "Strengths", Description: "What do we do well?", Colour: model.ColourGreen},
				{Title: "Weaknesses", Description: "Where do we fall short?", Colour: model.ColourRed},
				{Title: "Opportunities", Description: "What could we make the most of?", Colour: model.ColourBlue},
				{Title: "Threats", Description: "What could get in our way?", Colour: model.ColourAmber},
			},
		},
		{
			Key:         "pros-cons",
			Title:       "Pros and Cons",
			Description: "Weigh up a decision and note what is still unclear.",
			Columns: []Column{
				{Title: "Pros", Description: "What speaks in favour?", Colour: model.ColourGreen},
				{Title: "Cons", Description: "What speaks against?", Colour: model.ColourRed},
				{Title: "Questions", Description: "What do we still need to find out?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "six-thinking-hats",
			Title:       "Six Thinking Hats",
			Description: "Look at one topic from six angles, one hat at a time.",
			Columns: []Column{
				{Title: "Facts", Description: "What do we know for certain?", Colour: model.ColourGrey},
				{Title: "Feelings", Description: "What does your gut tell you?", Colour: model.ColourRed},
				{Title: "Risks", Description: "What could go wrong?", Colour: model.ColourPurple},
				{Title: "Benefits", Description: "What could go right?", Colour: model.ColourAmber},
				{Title: "Ideas", Description: "What new options can we think of?", Colour: model.ColourGreen},
				{Title: "Process", Description: "How should we approach this?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         "kudos",
			Title:       "Kudos",
			Description: "Thank and celebrate the people who made a difference.",
			Columns: []Column{
				{Title: "Thank you", Description: "Who would you like to thank?", Colour: model.ColourPink},
				{Title: "Well done", Description: "Whose work deserves praise?", Colour: model.ColourGreen},
				{Title: "Helped me", Description: "Who helped you out?", Colour: model.ColourTeal},
			},
		},
		{
			Key:         "daily-standup",
			Title:       "Daily Stand-up",
			Description: "Share progress, plans and blockers in a few minutes.",
			Columns: []Column{
				{Title: "Yesterday", Description: "What did you get done?", Colour: model.ColourGrey},
				{Title: "Today", Description: "What are you working on?", Colour: model.ColourBlue},
				{Title: "Blockers", Description: "What is in your way?", Colour: model.ColourRed},
			},
		},
		{
			Key:         "kanban",
			Title:       "Kanban",
			Description: "Track work as it moves from to do to done.",
			Columns: []Column{
				{Title: "To do", Description: "What is waiting to start?", Colour: model.ColourGrey},
				{Title: "Doing", Description: "What is in progress?", Colour: model.ColourAmber},
				{Title: "Done", Description: "What is finished?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "quarter-in-review",
			Title:       "Quarter in Review",
			Description: "Look back on the quarter and set the course for the next one.",
			Columns: []Column{
				{Title: "Wins", Description: "What did we achieve?", Colour: model.ColourGreen},
				{Title: "Misses", Description: "What did we not get to?", Colour: model.ColourRed},
				{Title: "Learnings", Description: "What did we learn?", Colour: model.ColourBlue},
				{Title: "Next quarter", Description: "What should we focus on next?", Colour: model.ColourPurple},
			},
		},
		{
			Key:         "expectations-reality",
			Title:       "Expectations and Reality",
			Description: "Compare what the team expected with what actually happened.",
			Columns: []Column{
				{Title: "Expected", Description: "What did we expect?", Colour: model.ColourBlue},
				{Title: "Happened", Description: "What actually happened?", Colour: model.ColourAmber},
				{Title: "Learned", Description: "What did the gap teach us?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "brainstorm",
			Title:       "Brainstorm",
			Description: "Gather ideas and questions, then agree what to do next.",
			Columns: []Column{
				{Title: "Ideas", Description: "What ideas do you have?", Colour: model.ColourPurple},
				{Title: "Questions", Description: "What do we need to answer?", Colour: model.ColourAmber},
				{Title: "Next steps", Description: "What should we do next?", Colour: model.ColourGreen},
			},
		},
		{
			Key:         "hopes-and-fears",
			Title:       "Hopes and Fears",
			Description: "Share hopes and worries before a project starts, and commit to what helps.",
			Columns: []Column{
				{Title: "Hopes", Description: "What are you hoping for?", Colour: model.ColourGreen},
				{Title: "Fears", Description: "What are you worried about?", Colour: model.ColourRed},
				{Title: "Commitments", Description: "What will we do to make it go well?", Colour: model.ColourBlue},
			},
		},
		{
			Key:         BlankKey,
			Title:       "Blank",
			Description: "Start with one column, then rename it and add the columns you need.",
			Columns: []Column{
				{Title: "Notes", Description: "Anything worth raising.", Colour: model.ColourGrey},
			},
		},
	}
)

func Lookup(key string) (Template, bool) {
	for _, template := range catalogue {
		if template.Key == key {
			template.Columns = slices.Clone(template.Columns)
			return template, true
		}
	}

	return Template{}, false
}

func All() []Template {
	templates := make([]Template, 0, len(catalogue))
	for _, template := range catalogue {
		template.Columns = slices.Clone(template.Columns)
		templates = append(templates, template)
	}

	return templates
}
