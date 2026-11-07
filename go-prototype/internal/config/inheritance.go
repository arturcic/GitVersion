package config

// Inherit retains explicit child values while filling nullable settings from
// one source configuration. Increment Inherit is represented by a nil pointer.
func (o Options) Inherit(parent Options) Options {
	if o.Increment == nil {
		o.Increment = parent.Increment
	}
	if o.Mode == "" {
		o.Mode = parent.Mode
	}
	if o.NullLabel {
		o.Label, o.NullLabel = parent.Label, parent.NullLabel
	}
	if o.CustomVersionFormat == nil {
		o.CustomVersionFormat = parent.CustomVersionFormat
	}
	if o.InheritPreventTagged {
		o.PreventTagged, o.InheritPreventTagged = parent.PreventTagged, parent.InheritPreventTagged
	}
	if o.InheritWeight {
		o.Weight, o.InheritWeight = parent.Weight, parent.InheritWeight
	}
	if o.Messages == nil {
		o.Messages = parent.Messages
	}
	for child, ancestor := range map[**bool]*bool{
		&o.IsMain: parent.IsMain, &o.IsRelease: parent.IsRelease,
		&o.TracksRelease: parent.TracksRelease, &o.TrackMergeTarget: parent.TrackMergeTarget,
		&o.PreventMerged: parent.PreventMerged, &o.PreventWhenMerged: parent.PreventWhenMerged,
		&o.TrackMergeMessage: parent.TrackMergeMessage,
	} {
		if *child == nil {
			*child = ancestor
		}
	}
	return o
}

// EffectiveOptions projects already inherited settings on the original branch.
func (c Config) EffectiveOptions(branch, sha string, options Options, inherited bool) (Effective, error) {
	kind, capture, err := c.Branch(branch)
	if err != nil {
		return Effective{}, err
	}
	return c.projectEffective(branch, kind, capture, sha, options, inherited)
}
