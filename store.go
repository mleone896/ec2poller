package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type StatusStore struct {
	status       map[string]string
	mu           sync.RWMutex
	file         *os.File
	isFileLoaded bool
	newWorld     map[string]string
}
type record struct {
	Key, Status string
}

func NewStatusStore(filename string) *StatusStore {
	s := &StatusStore{status: make(map[string]string)}
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal("StatusStore:", err)
	}
	s.file = f
	if err := s.load(); err != nil {
		log.Println("StatusStore:", err)
	}
	s.isFileLoaded = true
	return s
}

func (s *StatusStore) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status[key]
}

func (s *StatusStore) Set(key, status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, present := s.status[key]; present {
		return false
	}
	s.status[key] = status
	return true
}

func (s *StatusStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.status)
}

func (s *StatusStore) Check(key string) bool {

	if _, ok := s.status[key]; ok {
		fmt.Println(s.status[key])
		fmt.Println(ok)
		return true
	} else {
		fmt.Println(s.status[key])
		fmt.Println(ok)

		return false
	}

}
func (s *StatusStore) Put(status string) string {
	for {
		key := genKey(s.Count())
		if ok := s.Set(key, status); ok {
			if err := s.save(key, status); err != nil {
				log.Println("StatusStore:", err)
			}
			return key
		}
	}
	panic("shouldn't get here")
}

func (s *StatusStore) RemoveOldRecords(ec2Map map[string]string, status string) {
	for k, v := range s.status {
		if v == status {
			if ec2Map[k] == status {
			} else {
				delete(s.status, k)
			}
		}
	}
}

func (s *StatusStore) DataToFile(status string, c *Conn) {

	for k, v := range c.data {
		if v == status {
			if _, ok := s.status[k]; ok {
			} else {
				err := s.save(k, v)
				if err != nil {
					log.Printf("something went wrong save %s", k)
				}
			}
		}
	}
}
func (s *StatusStore) load() error {
	if _, err := s.file.Seek(0, 0); err != nil {
		return err
	}
	d := json.NewDecoder(s.file)
	var err error
	for err == nil {
		var r record
		if err = d.Decode(&r); err == nil {
			s.Set(r.Key, r.Status)
		}
	}
	if err == io.EOF {
		return nil
	}
	s.isFileLoaded = true
	return err
}

func (s *StatusStore) save(key, status string) error {
	e := json.NewEncoder(s.file)
	return e.Encode(record{key, status})
}
