package authcache

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/omc-college/management-system/pkg/pubsub"
	"github.com/omc-college/management-system/pkg/rbac/models"
)

type Cache struct {
	authRules
}

// NewCache inits Cache based on full history from MQ
func NewCache() {

}

func (cache *Cache) Update(envelope *pubsub.Envelope) error {
	switch envelope.Operation {
	case models.RoleOperationCreate:
		return cache.createRole(envelope.Payload)
	case models.RoleOperationUpdate:
		return cache.updateRole(envelope.Payload)
	case models.RoleOperationDelete:
		return cache.deleteRole(envelope.Payload)
	default:
		return fmt.Errorf("cannot recognize operation")
	}
}

func (cache *Cache) createRole(rawNewRole json.RawMessage) error {
	var newRole models.Role

	err := json.Unmarshal(rawNewRole, &newRole)
	if err != nil {
		return err
	}

	paramRegExp, err := regexp.Compile(`{\w+}`)
	if err != nil {
		return err
	}

	for _, newFeature := range newRole.Entries {
		for _, newEndpoint := range newFeature.Endpoints {
			var existingCacheRuleIndex int
			var existingCacheAuthMethodIndex int

			newPathRegExp := fmt.Sprintf("^%s$", paramRegExp.ReplaceAll([]byte(newEndpoint.Path), []byte("\\w+")))
			isPathRegExpExisting := false

			for cacheRuleIndex, cacheRule := range cache.Rules {
				if newPathRegExp == cacheRule.PathRegExp {
					isPathRegExpExisting = true
					existingCacheRuleIndex = cacheRuleIndex

					break
				}
			}

			if !isPathRegExpExisting {
				newAuthMethod := authMethod{
					Name:  newEndpoint.Method,
					Roles: []int{newRole.ID},
				}

				newAuthRule := authRule{
					PathRegExp: newPathRegExp,
					Methods:    []authMethod{newAuthMethod},
				}

				cache.Rules = append(cache.Rules, newAuthRule)

				continue
			}

			isMethodExisting := false

			for cacheAuthMethodID, cacheAuthMethod := range cache.Rules[existingCacheRuleIndex].Methods {
				if newEndpoint.Method == cacheAuthMethod.Name {
					isMethodExisting = true
					existingCacheAuthMethodIndex = cacheAuthMethodID
					break
				}
			}

			if !isMethodExisting {
				newAuthMethod := authMethod{
					Name:  newEndpoint.Method,
					Roles: []int{newRole.ID},
				}

				cache.Rules[existingCacheRuleIndex].Methods = append(cache.Rules[existingCacheRuleIndex].Methods, newAuthMethod)

				continue
			}

			for _, cacheRoleID := range cache.Rules[existingCacheRuleIndex].Methods[existingCacheAuthMethodIndex].Roles {
				if newRole.ID == cacheRoleID {
					return fmt.Errorf("cannot write already existing role to auth cache")
				}

				cache.Rules[existingCacheRuleIndex].Methods[existingCacheAuthMethodIndex].Roles = append(cache.Rules[existingCacheRuleIndex].Methods[existingCacheAuthMethodIndex].Roles, newRole.ID)
			}
		}
	}

	sort.SliceStable(cache.Rules, func(i, j int) bool {
		iLength := len(strings.Split(cache.Rules[i].PathRegExp, "/"))
		jLength := len(strings.Split(cache.Rules[j].PathRegExp, "/"))
		return iLength > jLength
	})

	return nil
}

func (cache *Cache) updateRole(rawNewRole json.RawMessage) error {
	var newRole models.Role

	err := json.Unmarshal(rawNewRole, &newRole)
	if err != nil {
		return err
	}

	RawNewRoleID, err := json.Marshal(newRole.ID)
	if err != nil {
		return err
	}

	err = cache.deleteRole(RawNewRoleID)
	if err != nil {
		return err
	}

	err = cache.createRole(rawNewRole)
	if err != nil {
		return err
	}

	return nil
}

func (cache *Cache) deleteRole(rawRoleID json.RawMessage) error {
	var roleID int

	err := json.Unmarshal(rawRoleID, &roleID)
	if err != nil {
		return err
	}

	isRuleDeleted := false

	for cacheRuleIndex, cacheRule := range cache.Rules {
		for cacheAuthMethodIndex, cacheAuthMethod := range cacheRule.Methods {
			for cacheRoleIDIndex, cacheRoleID := range cacheAuthMethod.Roles {
				if roleID == cacheRoleID {
					cache.Rules[cacheRuleIndex].Methods[cacheAuthMethodIndex].Roles = append(cacheAuthMethod.Roles[:cacheRoleIDIndex], cacheAuthMethod.Roles[cacheRoleIDIndex+1:]...)

					if len(cacheAuthMethod.Roles) == 0 {
						cache.Rules[cacheRuleIndex].Methods = append(cacheRule.Methods[:cacheAuthMethodIndex], cacheRule.Methods[cacheAuthMethodIndex+1:]...)

						if len(cacheRule.Methods) == 0 {
							isRuleDeleted = true

							cache.Rules = append(cache.Rules[:cacheRuleIndex], cache.Rules[cacheRuleIndex+1:]...)
						}
					}
				}
			}
		}
	}

	if isRuleDeleted {
		sort.SliceStable(cache.Rules, func(i, j int) bool {
			iLength := len(strings.Split(cache.Rules[i].PathRegExp, "/"))
			jLength := len(strings.Split(cache.Rules[j].PathRegExp, "/"))
			return iLength > jLength
		})
	}

	return nil
}
