package main

// cmd_init.go implements the offline armor init provisioning command. Secret
// values stay out of argv: setup flags are non-secret, while credential values
// come from the environment, protected files, or interactive prompts.

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	initDefaultEnvFile = ".env"
	initEnvExample     = ".env.example"
)

var (
	initEnvFileFlag            string
	initNonInteractiveFlag     bool
	initRegionFlag             string
	initBucketFlag             string
	initB2AccessKeyIDFlag      string
	initCFDomainFlag           string
	initAuthAccessKeyFlag      string
	initB2CredentialFileFlag   string
	initAuthCredentialFileFlag string
)

func init() {
	flags := flag.NewFlagSet("init", flag.ExitOnError)
	flags.StringVar(&initEnvFileFlag, "env-file", initDefaultEnvFile, "Environment file to create or update")
	flags.BoolVar(&initNonInteractiveFlag, "non-interactive", false, "Do not prompt; leave unspecified optional values empty")
	flags.StringVar(&initRegionFlag, "region", envFlagDefault("ARMOR_B2_REGION"), "B2 region")
	flags.StringVar(&initRegionFlag, "b2-region", envFlagDefault("ARMOR_B2_REGION"), "DEPRECATED alias of -region")
	flags.StringVar(&initBucketFlag, "bucket", envFlagDefault("ARMOR_BUCKET"), "B2 bucket name")
	flags.StringVar(&initB2AccessKeyIDFlag, "b2-access-key-id", envFlagDefault("ARMOR_B2_ACCESS_KEY_ID"), "B2 application key ID")
	flags.StringVar(&initCFDomainFlag, "cf-domain", envFlagDefault("ARMOR_CF_DOMAIN"), "Cloudflare read hostname")
	flags.StringVar(&initAuthAccessKeyFlag, "auth-access-key", envFlagDefault("ARMOR_AUTH_ACCESS_KEY"), "ARMOR client access key")
	flags.StringVar(&initB2CredentialFileFlag, "b2-secret-access-key-file", "", "Read the B2 application key from this file")
	flags.StringVar(&initAuthCredentialFileFlag, "auth-secret-key-file", "", "Read the ARMOR client secret from this file")
	registerCommand(Command{
		Name:        "init",
		Description: "Create or update a mode-600 .env with fresh encryption and admin secrets",
		Flags:       flags,
		Func:        runInit,
	})
}

type initValues struct {
	region         string
	bucket         string
	b2AccessKeyID  string
	b2Credential   string
	cfDomain       string
	authAccessKey  string
	authCredential string
}

var initEnvKeys = []string{
	"ARMOR_B2_REGION",
	"ARMOR_B2_ACCESS_KEY_ID",
	"ARMOR_B2_SECRET_ACCESS_KEY",
	"ARMOR_BUCKET",
	"ARMOR_CF_DOMAIN",
	"ARMOR_MEK",
	"ARMOR_ADMIN_TOKEN",
	"ARMOR_AUTH_ACCESS_KEY",
	"ARMOR_AUTH_SECRET_KEY",
}

func runInit(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", fs.Args())
		fmt.Fprintln(os.Stderr, "Usage: armor init [flags]")
		exit(2)
	}
	printDeprecatedFlagPointers(fs, map[string]string{"b2-region": "-region"})

	values, err := initValuesFromFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: armor init: %v\n", err)
		exit(1)
	}
	interactive := !initNonInteractiveFlag && initStreamsAreInteractive()
	if interactive {
		if err := promptForInitValues(&values, os.Stdin, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: armor init: %v\n", err)
			exit(1)
		}
	}
	if err := initializeEnvFile(initEnvFileFlag, values); err != nil {
		fmt.Fprintf(os.Stderr, "Error: armor init: %v\n", err)
		exit(1)
	}

	fmt.Fprintln(os.Stdout, "Initialized environment file with mode 0600.")
	fmt.Fprintln(os.Stdout, "WARNING: back up ARMOR_MEK securely before writing objects; losing it makes existing objects unreadable.")
	if !interactive && initValuesHaveUnsetOptional(values) {
		fmt.Fprintln(os.Stdout, "Some optional service values remain empty; fill them in before starting ARMOR.")
	}
}

func initValuesFromFlags() (initValues, error) {
	values := initValues{
		region:        initRegionFlag,
		bucket:        initBucketFlag,
		b2AccessKeyID: initB2AccessKeyIDFlag,
		cfDomain:      initCFDomainFlag,
		authAccessKey: initAuthAccessKeyFlag,
	}
	var err error
	values.b2Credential, err = readInitInput(initEnvKeys[2], initB2CredentialFileFlag)
	if err != nil {
		return initValues{}, err
	}
	values.authCredential, err = readInitInput(initEnvKeys[8], initAuthCredentialFileFlag)
	if err != nil {
		return initValues{}, err
	}
	return values, nil
}

func readInitInput(envName, fileName string) (string, error) {
	if fileName == "" {
		return os.Getenv(envName), nil
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return "", fmt.Errorf("read environment input file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func initStreamsAreInteractive() bool {
	return initFileIsTerminal(os.Stdin) && initFileIsTerminal(os.Stderr)
}

func initFileIsTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func promptForInitValues(values *initValues, input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	var err error
	if values.region, err = promptIfEmpty(reader, output, "B2 region", values.region); err != nil {
		return err
	}
	if values.bucket, err = promptIfEmpty(reader, output, "B2 bucket", values.bucket); err != nil {
		return err
	}
	if values.b2AccessKeyID, err = promptIfEmpty(reader, output, "B2 application key ID", values.b2AccessKeyID); err != nil {
		return err
	}
	if values.b2Credential, err = promptIfEmpty(reader, output, "B2 application key (input is not printed by ARMOR)", values.b2Credential); err != nil {
		return err
	}
	if values.cfDomain, err = promptIfEmpty(reader, output, "Cloudflare hostname (without https://)", values.cfDomain); err != nil {
		return err
	}
	if values.authAccessKey, err = promptIfEmpty(reader, output, "ARMOR client access key", values.authAccessKey); err != nil {
		return err
	}
	if values.authCredential, err = promptIfEmpty(reader, output, "ARMOR client secret (input is not printed by ARMOR)", values.authCredential); err != nil {
		return err
	}
	return nil
}

func promptIfEmpty(reader *bufio.Reader, output io.Writer, label, value string) (string, error) {
	if value != "" {
		return value, nil
	}
	fmt.Fprintf(output, "%s: ", label)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func initValuesHaveUnsetOptional(values initValues) bool {
	return values.region == "" || values.bucket == "" || values.b2AccessKeyID == "" ||
		values.b2Credential == "" || values.cfDomain == "" || values.authAccessKey == "" || values.authCredential == ""
}

func initializeEnvFile(path string, values initValues) error {
	contents, err := readInitEnvTemplate(path)
	if err != nil {
		return err
	}
	updates := map[string]string{
		initEnvKeys[0]: values.region,
		initEnvKeys[1]: values.b2AccessKeyID,
		initEnvKeys[2]: values.b2Credential,
		initEnvKeys[3]: values.bucket,
		initEnvKeys[4]: values.cfDomain,
		initEnvKeys[7]: values.authAccessKey,
		initEnvKeys[8]: values.authCredential,
	}
	first, second, err := initSecrets()
	if err != nil {
		return err
	}
	updates[initEnvKeys[5]] = first
	updates[initEnvKeys[6]] = second

	for _, key := range initEnvKeys {
		contents, err = setEnvValue(contents, key, updates[key])
		if err != nil {
			return err
		}
	}
	return writeInitEnvFile(path, []byte(contents))
}

func readInitEnvTemplate(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read environment file: %w", err)
	}
	data, err = os.ReadFile(initEnvExample)
	if err == nil {
		return string(data), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read environment template: %w", err)
	}
	return "# ARMOR environment generated by armor init.\n", nil
}

func initSecrets() (string, string, error) {
	first, err := randomHexValue()
	if err != nil {
		return "", "", fmt.Errorf("generate encryption key: %w", err)
	}
	second, err := randomHexValue()
	if err != nil {
		return "", "", fmt.Errorf("generate admin token: %w", err)
	}
	return first, second, nil
}

func randomHexValue() (string, error) {
	var raw [32]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// setEnvValue updates only an empty assignment. A populated value, including
// its original quoting and spacing, is preserved exactly.
func setEnvValue(contents, key, value string) (string, error) {
	lines := strings.SplitAfter(contents, "\n")
	for i, line := range lines {
		prefix, existing, found := envAssignment(line, key)
		if !found {
			continue
		}
		if existing != "" {
			return contents, nil
		}
		newline := ""
		if strings.HasSuffix(line, "\n") {
			newline = "\n"
		}
		lines[i] = prefix + value + newline
		return strings.Join(lines, ""), nil
	}
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	return contents + key + "=" + value + "\n", nil
}

func envAssignment(line, key string) (prefix, value string, found bool) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	}
	nameEnd := strings.IndexByte(trimmed, '=')
	if nameEnd < 0 || strings.TrimSpace(trimmed[:nameEnd]) != key {
		return "", "", false
	}
	leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	namePrefix := leading + key + "="
	rawValue := strings.TrimSpace(trimmed[nameEnd+1:])
	return namePrefix, dotenvValue(rawValue), true
}

func dotenvValue(value string) string {
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		for i := 1; i < len(value); i++ {
			if value[i] != '"' || value[i-1] == '\\' {
				continue
			}
			if parsed, err := strconv.Unquote(value[:i+1]); err == nil {
				return parsed
			}
			break
		}
	}
	if value[0] == '\'' {
		if end := strings.IndexByte(value[1:], '\''); end >= 0 {
			return value[1 : end+1]
		}
	}
	return value
}

func writeInitEnvFile(path string, contents []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".armor-init-*")
	if err != nil {
		return fmt.Errorf("create temporary environment file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("set environment file permissions: %w", err)
	}
	if _, err := tmp.Write(contents); err != nil {
		tmp.Close()
		return fmt.Errorf("write environment file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync environment file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close environment file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install environment file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set environment file permissions: %w", err)
	}
	return nil
}
