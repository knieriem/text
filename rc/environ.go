package rc

// An EnvMap contains environment variables.
type EnvMap map[string][]string

// Insert copies all elements from src into m.
func (m EnvMap) Insert(src EnvMap) {
	for name, val := range src {
		m[name] = val
	}
}

type EnvStack []EnvMap

// Get the value of a variable from the topmost EnvMap of s.
func (s EnvStack) Get(name string) (value []string) {
	for i := s.iLast(); i >= 0; i-- {
		v, ok := s[i][name]
		if ok {
			value = v
			break
		}
	}
	return
}

// Set the value of a variable in the topmost EnvMap of s.
func (s *EnvStack) Set(name string, value []string) {
	if i := s.iLast(); i >= 0 {
		(*s)[i][name] = value
	}
}

// Push pushes m onto the EnvStack s.
func (s *EnvStack) Push(m EnvMap) {
	if m == nil {
		m = make(EnvMap, 8)
	}
	*s = append(*s, m)
}

// Pop removes the topmost EnvMap from s.
func (s *EnvStack) Pop() {
	*s = (*s)[:s.iLast()]

}

func (s EnvStack) iLast() int {
	return len(s) - 1
}

// Insert copies all elements from m into the topmost EnvMap of s.
func (s EnvStack) Insert(m EnvMap) {
	i := s.iLast()
	dest := s[i]
	if dest == nil {
		s[i] = m
	} else {
		dest.Insert(m)
	}
}
