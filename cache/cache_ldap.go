package cache

import (
	"errors"
	"r3/ldap"
	"r3/types"
	"sync"
)

var (
	ldap_mx   sync.Mutex
	ldapIdMap map[int32]types.Ldap
)

func GetLdapIdMap() map[int32]types.Ldap {
	ldap_mx.Lock()
	defer ldap_mx.Unlock()
	return ldapIdMap
}

func GetLdap(id int32) (types.Ldap, error) {
	ldap_mx.Lock()
	defer ldap_mx.Unlock()

	ldap, exists := ldapIdMap[id]
	if !exists {
		return ldap, errors.New("unknown LDAP connection")
	}
	return ldap, nil
}

func LoadLdapMap() error {

	ldap_mx.Lock()
	defer ldap_mx.Unlock()

	ldaps, err := ldap.Get()
	if err != nil {
		return err
	}

	ldapIdMap = make(map[int32]types.Ldap)

	for _, ldap := range ldaps {
		ldapIdMap[ldap.Id] = ldap
	}
	return nil
}
