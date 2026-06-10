package motofb

import (
	"encoding/json"
	"fmt"
	"os"

	fberr "github.com/motovax/motofb/errors"
)

// AccountSpec describes one managed Facebook account.
// Sessions are loaded from SQLite; import cookies first with ImportCookies.
type AccountSpec struct {
	ID        string `json:"id"`
	UserAgent string `json:"user_agent,omitempty"`
	ProxyURL  string `json:"proxy,omitempty"`
	Online    *bool  `json:"online,omitempty"`
}

func (a AccountSpec) options() []Option {
	var opts []Option
	if a.UserAgent != "" {
		opts = append(opts, WithUserAgent(a.UserAgent))
	}
	if a.ProxyURL != "" {
		opts = append(opts, WithProxy(a.ProxyURL))
	}
	if a.Online != nil {
		opts = append(opts, WithOnline(*a.Online))
	}
	return opts
}

// accountsFile is the JSON schema for bulk account loading.
type accountsFile struct {
	Accounts []AccountSpec `json:"accounts"`
}

// LoadAccountSpecs reads account definitions from a JSON file.
func LoadAccountSpecs(path string) ([]AccountSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fberr.Wrap("LoadAccountSpecs", "read file", err)
	}
	var file accountsFile
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, fberr.Wrap("LoadAccountSpecs", "parse json", err)
	}
	if len(file.Accounts) == 0 {
		return nil, fberr.New("LoadAccountSpecs", "no accounts defined")
	}
	for i, a := range file.Accounts {
		if a.ID == "" {
			return nil, fberr.New("LoadAccountSpecs", fmt.Sprintf("accounts[%d]: missing id", i))
		}
	}
	return file.Accounts, nil
}