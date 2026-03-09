package policy

// Policy represents parsed OPA Rego policy information.
type Policy struct {
	File       string
	Rules      []AllowRule
	Ownerships []OwnershipMapping
}

// AllowRule represents an extracted (action, resource) pair from an allow rule.
type AllowRule struct {
	Actions         []string // single or set of actions
	Resource        string
	UsesOwner       bool // references input.resource_owner
	UsesRole        bool // references input.user.role
	RoleValue       string
	SourceLine      int
}

// OwnershipMapping represents a @ownership annotation.
type OwnershipMapping struct {
	Resource  string
	Table     string
	Column    string
	JoinTable string // empty if direct lookup
	JoinFK    string // empty if direct lookup
}

// ActionResourcePairs returns all (action, resource) pairs from the policy.
func (p *Policy) ActionResourcePairs() [][2]string {
	var pairs [][2]string
	for _, r := range p.Rules {
		for _, a := range r.Actions {
			pairs = append(pairs, [2]string{a, r.Resource})
		}
	}
	return pairs
}

// OwnershipFor returns the ownership mapping for a resource, if any.
func (p *Policy) OwnershipFor(resource string) (OwnershipMapping, bool) {
	for _, o := range p.Ownerships {
		if o.Resource == resource {
			return o, true
		}
	}
	return OwnershipMapping{}, false
}

// ResourcesUsingOwner returns resources that reference input.resource_owner in allow rules.
func (p *Policy) ResourcesUsingOwner() []string {
	seen := make(map[string]bool)
	for _, r := range p.Rules {
		if r.UsesOwner {
			seen[r.Resource] = true
		}
	}
	var result []string
	for res := range seen {
		result = append(result, res)
	}
	return result
}
