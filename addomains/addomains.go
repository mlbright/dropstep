package addomains

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	AdList              = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
	AdDomainInterval    = 18000
	RequestsUntilUpdate = 1000
)

type adGenerator struct {
	requests uint
	bytes    uint
}

type AdDomainDb struct {
	RwLock    sync.RWMutex
	AdDomains map[string]*adGenerator
	Requests  uint64
	Ticker    time.Ticker
}

func NewAdDomains() *AdDomainDb {

	db := &AdDomainDb{
		AdDomains: make(map[string]*adGenerator),
	}

	return db
}

func (db *AdDomainDb) GetAdDomains() error {

	response, err := http.Get(AdList)
	if err != nil {
		return fmt.Errorf("could not obtain ad domains: %w", err)
	}
	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)

	domainDb, err := os.Create("ad-domains.txt")
	if err != nil {
		return fmt.Errorf("could not open ad domain file for writing: %w", err)
	}
	defer domainDb.Close()

	wb := bufio.NewWriter(domainDb)

	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "0.0.0.0") {
			fields := strings.Split(t, " ")
			domain := strings.Trim(fields[1], " ")
			if domain != "0.0.0.0" {
				wb.WriteString(domain)
				wb.WriteString("\n")

				// This is a lot of locking and unlocking.
				// However if it saves reading an ad, it's worth it.
				db.RwLock.RLock()
				_, exists := db.AdDomains[domain]
				db.RwLock.RUnlock()

				if !exists {
					db.RwLock.Lock()
					db.AdDomains[domain] = &adGenerator{}
					db.RwLock.Unlock()
				}
			}
		}
	}

	wb.Flush()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("processing ad domain list failed: %w", err)
	}

	return nil
}
