package key

import "fmt"

type Binding struct {
	keys     []string
	help     Help
	disabled bool
}

type BindingOpt func(*Binding)

func NewBinding(opts ...BindingOpt) Binding {
	b := &Binding{}
	for _, opt := range opts {
		opt(b)
	}
	return *b
}

func WithKeys(keys ...string) BindingOpt {
	return func(b *Binding) {
		b.keys = keys
	}
}

func WithHelp(key, desc string) BindingOpt {
	return func(b *Binding) {
		b.help = Help{Key: key, Desc: desc}
	}
}

func WithDisabled() BindingOpt {
	return func(b *Binding) {
		b.disabled = true
	}
}

func (b *Binding) SetKeys(keys ...string) {
	b.keys = keys
}

func (b Binding) Keys() []string {
	return b.keys
}

func (b *Binding) SetHelp(key, desc string) {
	b.help = Help{Key: key, Desc: desc}
}

func (b Binding) Help() Help {
	return b.help
}

func (b Binding) Enabled() bool {
	return !b.disabled && b.keys != nil
}

func (b *Binding) SetEnabled(v bool) {
	b.disabled = !v
}

func (b *Binding) Unbind() {
	b.keys = nil
	b.help = Help{}
}

type Help struct {
	Key  string
	Desc string
}

func Matches[Key fmt.Stringer](k Key, b ...Binding) bool {
	keys := k.String()
	for _, binding := range b {
		for _, v := range binding.keys {
			if keys == v && binding.Enabled() {
				return true
			}
		}
	}
	return false
}
