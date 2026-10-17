package db

import "github.com/fcavalcantirj/solvr/internal/traffic"

// TrafficLabel is who a stored row came from (SPEC.md 27.6): its traffic_class and
// traffic_reason. The zero value stores unclassified with no reason, which is what a confirmed
// server step, and every row stored before classification existed, carries.
type TrafficLabel struct {
	Class  traffic.Class
	Reason traffic.Reason
}

// LabelFor classifies a request's signals into the label its row stores.
func LabelFor(s traffic.Signals) TrafficLabel {
	class, reason := traffic.Classify(s)
	return TrafficLabel{Class: class, Reason: reason}
}

// columns returns the (traffic_class, traffic_reason) values to store.
func (l TrafficLabel) columns() (class string, reason any) {
	class = string(l.Class)
	if class == "" {
		class = string(traffic.Unclassified)
	}
	if l.Reason == "" {
		return class, nil
	}
	return class, string(l.Reason)
}
