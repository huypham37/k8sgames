package game

import "fmt"

type Probe struct {
	Args      []string
	Equals    string
	Unordered []string
	AtLeast   int
	NotEmpty  bool
	Pending   string
}

type Challenge struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Chapter   string     `json:"chapter"`
	Objective string     `json:"objective"`
	Hint      string     `json:"hint"`
	Manifest  string     `json:"-"`
	Setup     [][]string `json:"-"`
	Probes    []Probe    `json:"-"`
}

type Catalog struct {
	items []Challenge
	byID  map[string]Challenge
}

func NewCatalog() *Catalog {
	items := allChallenges()
	byID := make(map[string]Challenge, len(items))
	for _, challenge := range items {
		byID[challenge.ID] = challenge
	}
	return &Catalog{items: items, byID: byID}
}

func (c *Catalog) Find(id string) (Challenge, error) {
	challenge, ok := c.byID[id]
	if !ok {
		return Challenge{}, fmt.Errorf("unknown challenge %q", id)
	}
	return challenge, nil
}

func (c *Catalog) All() []Challenge {
	return append([]Challenge(nil), c.items...)
}

func allChallenges() []Challenge {
	groups := [][]Challenge{
		foundationChallenges(), workloadChallenges(), networkingChallenges(),
		stateChallenges(), productionChallenges(),
	}
	var challenges []Challenge
	for _, group := range groups {
		challenges = append(challenges, group...)
	}
	return challenges
}

func probe(args []string, equals, pending string) Probe {
	return Probe{Args: args, Equals: equals, Pending: pending}
}

func nonEmptyProbe(args []string, pending string) Probe {
	return Probe{Args: args, NotEmpty: true, Pending: pending}
}

func unorderedProbe(args, values []string, pending string) Probe {
	return Probe{Args: args, Unordered: values, Pending: pending}
}

func atLeastProbe(args []string, value int, pending string) Probe {
	return Probe{Args: args, AtLeast: value, Pending: pending}
}
