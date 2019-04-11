package amalgam

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const tagName = "amalgam"

// Amalgam is the configuration loader object.
type Amalgam struct {
	configFile        string
	configObj         interface{}
	preventConfigFlag bool
	envPrefix         string
	flagNameFunc      func(string) string
	flagSet           *pflag.FlagSet
	viper             *viper.Viper
}

// Option is an option function, which operates on an Amalgam instance.
type Option func(a *Amalgam)

// PreventConfigFlag skips the adding of a --config flag to the flagset.
func PreventConfigFlag(a *Amalgam) {
	a.preventConfigFlag = true
}

// WithEnvPrefix allows the caller to specify a prefix to use for populating
// the config from environment variables.
func WithEnvPrefix(prefix string) func(*Amalgam) {
	return func(a *Amalgam) {
		a.envPrefix = prefix
	}
}

// WithFlagSet allows the caller to specify the pflag FlagSet to use.
func WithFlagSet(fs *pflag.FlagSet) func(*Amalgam) {
	return func(a *Amalgam) {
		a.flagSet = fs
	}
}

// WithFlagNameFunc allows the caller to specify a function to determine
// the flag name from the config key.
func WithFlagNameFunc(fn func(string) string) func(*Amalgam) {
	return func(a *Amalgam) {
		a.flagNameFunc = fn
	}
}

// WithDefaultConfigFile allows the caller to specify the config file to
// load.  This value can be overridden by the --config flag, if PreventConfigFlag
// has not been specified.
func WithDefaultConfigFile(configFile string) func(*Amalgam) {
	return func(a *Amalgam) {
		a.configFile = configFile
	}
}

type fieldInfo struct {
	value       reflect.Value
	description string
	flagName    string
}

var defaultFlagNameFunc = func(name string) string {
	replacements := map[string]string{
		"([a-z])([A-Z])":      "$1-$2",
		"([A-Z])([A-Z][a-z])": "$1-$2",
	}
	for pattern, replace := range replacements {
		r := regexp.MustCompile(pattern)
		name = r.ReplaceAllString(name, replace)
	}
	name = strings.Replace(name, ".", "-", -1)
	name = strings.Replace(name, "_", "-", -1)
	name = strings.ToLower(name)

	return name
}

type fieldMap map[string]fieldInfo

// New returns a new, populated Amalgam object.  If PreventConfigFlag was
// not specified, it also adds a --config flag to the flagset.
func New(configObj interface{}, options ...Option) (*Amalgam, error) {
	a := new(Amalgam)
	a.configObj = configObj
	a.flagNameFunc = defaultFlagNameFunc

	for _, opt := range options {
		opt(a)
	}

	if a.flagSet == nil {
		a.flagSet = pflag.CommandLine
	}
	if !a.preventConfigFlag {
		a.flagSet.StringVarP(&a.configFile, "config", "c", a.configFile, "config file to use")
	}

	a.viper = viper.New()
	if a.envPrefix != "" {
		a.viper.SetEnvPrefix(a.envPrefix)
	}
	a.viper.AutomaticEnv()
	// This replacer should really be a function, to match the flag
	// name function, but argh, it doesn't use an interface.
	// This means that case changes or special characters in key names
	// don't get separated with underscores.
	replacer := strings.NewReplacer(".", "_", "-", "_")
	a.viper.SetEnvKeyReplacer(replacer)

	if err := a.parse(a.configObj); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Amalgam) parse(configObj interface{}) error {
	val := reflect.ValueOf(configObj)
	if val.Kind() != reflect.Ptr {
		return errors.New("config object must be a pointer")
	}
	val = val.Elem()
	if !val.CanAddr() {
		return errors.New("config object must be addressable (a pointer)")
	}

	fm, err := structFieldTypes(val, "")
	if err != nil {
		return err
	}

	fs := a.flagSet
	tokenIP := net.ParseIP("127.0.0.1")

	for field, info := range fm {
		name := info.flagName
		val := info.value.Interface()
		a.viper.SetDefault(field, val)

		if name == "" {
			name = a.flagNameFunc(field)
		} else if name == "-" {
			continue
		}

		switch info.value.Type() {
		case reflect.TypeOf(tokenIP):
			fs.IP(name, val.(net.IP), info.description)
		case reflect.TypeOf(tokenIP.DefaultMask()):
			fs.IPMask(name, val.(net.IPMask), info.description)
		default:
			switch info.value.Kind() {
			case reflect.String:
				fs.String(name, val.(string), info.description)
			case reflect.Bool:
				fs.Bool(name, val.(bool), info.description)
			case reflect.Int:
				fs.Int(name, val.(int), info.description)
			case reflect.Int8:
				fs.Int8(name, val.(int8), info.description)
			case reflect.Int16:
				fs.Int16(name, val.(int16), info.description)
			case reflect.Int32:
				fs.Int32(name, val.(int32), info.description)
			case reflect.Int64:
				if info.value.Type() == reflect.TypeOf(time.Duration(10)) {
					fs.Duration(name, val.(time.Duration), info.description)
				} else {
					fs.Int64(name, val.(int64), info.description)
				}
			case reflect.Uint:
				fs.Uint(name, val.(uint), info.description)
			case reflect.Uint8:
				fs.Uint8(name, val.(uint8), info.description)
			case reflect.Uint16:
				fs.Uint16(name, val.(uint16), info.description)
			case reflect.Uint32:
				fs.Uint32(name, val.(uint32), info.description)
			case reflect.Uint64:
				fs.Uint64(name, val.(uint64), info.description)
			case reflect.Float32:
				fs.Float32(name, val.(float32), info.description)
			case reflect.Float64:
				fs.Float64(name, val.(float64), info.description)
			case reflect.Slice:
				elem := info.value.Type().Elem()
				switch elem {
				case reflect.TypeOf(net.ParseIP("127.0.0.1")):
					fs.IPSlice(name, info.value.Interface().([]net.IP), info.description)
				default:
					switch elem.Kind() {
					case reflect.String:
						fs.StringSlice(name, val.([]string), info.description)
					case reflect.Bool:
						fs.BoolSlice(name, val.([]bool), info.description)
					case reflect.Int:
						fs.IntSlice(name, val.([]int), info.description)
					case reflect.Int64:
						if elem == reflect.TypeOf(time.Duration(10)) {
							fs.DurationSlice(name, val.([]time.Duration), info.description)
						}
					case reflect.Uint:
						fs.UintSlice(name, val.([]uint), info.description)
					case reflect.Uint8:
						// this is probably a []byte, so let's treat it as such
						if elem == reflect.TypeOf(net.ParseIP("127.0.0.1")) {
							fs.IP(name, val.(net.IP), info.description)
						} else {
							fs.BytesHex(name, val.([]byte), info.description)
						}
					}
				}
			}
		}

		if flag := fs.Lookup(name); flag != nil {
			a.viper.BindPFlag(field, flag)
		}
	}

	return nil
}

// LoadFile hydrates the config from the config file.  This is either the value
// of the configFile property, or a value specified by the --config flag (if allowed).
func (a *Amalgam) LoadFile() error {
	if !a.flagSet.Parsed() {
		a.flagSet.Parse(os.Args[1:])
	}

	// If no config file is specified, load from a blank file
	// to allow flags to update config object.
	if a.configFile == "" {
		return a.Load(bytes.NewReader(nil))
	}

	a.viper.SetConfigFile(a.configFile)

	if err := a.viper.ReadInConfig(); err != nil {
		return err
	}

	if err := a.viper.Unmarshal(a.configObj); err != nil {
		return err
	}

	return nil
}

// Load hydrates the config from an io.Reader.
func (a *Amalgam) Load(r io.Reader) error {
	if !a.flagSet.Parsed() {
		a.flagSet.Parse(os.Args[1:])
	}

	if err := a.viper.ReadConfig(r); err != nil {
		return err
	}

	if err := a.viper.Unmarshal(a.configObj); err != nil {
		return err
	}

	return nil
}

func structFieldTypes(val reflect.Value, prefix string) (fieldMap, error) {
	types := make(fieldMap)

	if val.Kind() != reflect.Struct {
		return nil, errors.New("reflected object must be a struct value")
	}

	for i := 0; i < val.NumField(); i++ {
		structField := val.Type().Field(i)
		tagParts := strings.SplitN(structField.Tag.Get(tagName), ",", 2)
		flagName := tagParts[0]
		description := ""
		if len(tagParts) > 1 {
			description = tagParts[1]
		}
		fieldValue := val.Field(i)
		if fieldValue.Kind() == reflect.Ptr {
			fieldValue = fieldValue.Elem()
		}

		fieldInfo := fieldInfo{
			value:       fieldValue,
			description: description,
			flagName:    flagName,
		}

		if !fieldInfo.value.CanInterface() {
			// we won't be able to use it anyway, so we'll
			// just skip over
			continue
		}

		fieldName := structField.Name
		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		if fieldValue.Type().Kind() == reflect.Struct {
			fieldTypes, err := structFieldTypes(fieldValue, fieldName)
			if err != nil {
				return nil, err
			}
			for name, ft := range fieldTypes {
				types[name] = ft
			}
		} else {
			types[fieldName] = fieldInfo
		}
	}

	return types, nil
}
