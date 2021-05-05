package hugorm

// if doing special adapters for special types of flags, this might need to go in
// special module/sub-package

//NOFLAGYET import (
//NOFLAGYET 	"fmt"
//NOFLAGYET 	"github.com/spf13/pflag"
//NOFLAGYET )
//NOFLAGYET
//NOFLAGYET // pflagValueSet is a wrapper around *pflag.ValueSet
//NOFLAGYET // that implements FlagValueSet.
//NOFLAGYET type pflagValueSet struct {
//NOFLAGYET 	flags *pflag.FlagSet
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // VisitAll iterates over all *pflag.Flag inside the *pflag.FlagSet.
//NOFLAGYET func (p pflagValueSet) VisitAll(fn func(flag FlagValue)) {
//NOFLAGYET 	p.flags.VisitAll(func(flag *pflag.Flag) {
//NOFLAGYET 		fn(pflagValue{flag})
//NOFLAGYET 	})
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // pflagValue is a wrapper aroung *pflag.flag
//NOFLAGYET // that implements FlagValue
//NOFLAGYET type pflagValue struct {
//NOFLAGYET 	flag *pflag.Flag
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // ExplicitlyGiven returns whether the flag has changes or not.
//NOFLAGYET func (p pflagValue) ExplicitlyGiven() bool {
//NOFLAGYET 	return p.flag.Changed
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // Name returns the name of the flag.
//NOFLAGYET func (p pflagValue) Name() string {
//NOFLAGYET 	return p.flag.Name
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // ValueString returns the value of the flag as a string.
//NOFLAGYET func (p pflagValue) ValueString() string {
//NOFLAGYET 	return p.flag.Value.String()
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // ValueType returns the type of the flag as a string.
//NOFLAGYET func (p pflagValue) ValueType() string {
//NOFLAGYET 	return p.flag.Value.Type()
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET //=============================================================================
//NOFLAGYET
//NOFLAGYET // BindPFlag binds a specific key to a pflag
//NOFLAGYET func BindPFlag(key string, flag *pflag.Flag) error { return hg.BindPFlag(key, flag) }
//NOFLAGYET
//NOFLAGYET func (h *Hugorm) BindPFlag(key string, flag *pflag.Flag) error {
//NOFLAGYET 	if flag == nil {
//NOFLAGYET 		return fmt.Errorf("flag for %q is nil", key)
//NOFLAGYET 	}
//NOFLAGYET 	return h.BindFlagValue(key, pflagValue{flag})
//NOFLAGYET }
//NOFLAGYET
//NOFLAGYET // BindPFlags binds a full flag set to the configuration, using each flag's long
//NOFLAGYET // name as the config key.
//NOFLAGYET func BindPFlags(flags *pflag.FlagSet) error { return hg.BindPFlags(flags) }
//NOFLAGYET
//NOFLAGYET func (h *Hugorm) BindPFlags(flags *pflag.FlagSet) error {
//NOFLAGYET 	return h.BindFlagValues(pflagValueSet{flags})
//NOFLAGYET }
//NOFLAGYET
