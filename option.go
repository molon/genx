package genx

type Option func(*Config) error

func Extensions(extensions ...Extension) Option {
	return func(conf *Config) error {
		for _, ex := range extensions {
			conf.Extensions = append(conf.Extensions, ex)
		}
		return nil
	}
}
