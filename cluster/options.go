package cluster

import "github.com/georgysavva/scany/v2/pgxscan"

// Options for cluster.
type Options struct {
	Picker  ConnPicker
	ScanAPI *pgxscan.API
}

// Option func.
type Option func(*Options)

// WithConnPicker sets connection picker for Select and Get.
func WithConnPicker(picker ConnPicker) Option {
	return func(o *Options) {
		if picker != nil {
			o.Picker = picker
		}
	}
}

// WithScanAPI sets custom pgxscan api.
func WithScanAPI(api *pgxscan.API) Option {
	return func(o *Options) {
		if api != nil {
			o.ScanAPI = api
		}
	}
}
