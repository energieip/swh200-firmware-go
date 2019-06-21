package database

import (
	"github.com/energieip/common-components-go/pkg/duser"
	"github.com/energieip/common-components-go/pkg/pconst"
)

//SaveUserConfig dump user config in database
func SaveUserConfig(db Database, cfg duser.UserAccess) error {
	criteria := make(map[string]interface{})
	criteria["UserHash"] = cfg.UserHash
	return SaveOnUpdateObject(db, cfg, pconst.DbConfig, AccessTable, criteria)
}

//RemoveUserConfig remove user config in database
func RemoveUserConfig(db Database, userHash string) error {
	criteria := make(map[string]interface{})
	criteria["UserHash"] = userHash
	return db.DeleteRecord(pconst.DbConfig, AccessTable, criteria)
}

//GetUser retrive user from the database
func GetUser(db Database, userHash string) *duser.UserAccess {
	criteria := make(map[string]interface{})
	criteria["UserHash"] = userHash
	stored, err := db.GetRecord(pconst.DbConfig, AccessTable, criteria)
	if err != nil || stored == nil {
		return nil
	}
	user, err := duser.ToUserAccess(stored)
	if err != nil {
		return nil
	}
	return user
}

//GetUserConfigs get user Config for a given group list
func GetUserConfigs(db Database, groups map[int]bool) map[string]duser.UserAccess {
	users := make(map[string]duser.UserAccess)
	stored, err := db.FetchAllRecords(pconst.DbConfig, AccessTable)
	if err != nil || stored == nil {
		return users
	}
	for _, val := range stored {
		usr, err := duser.ToUserAccess(val)
		if err != nil || usr == nil {
			continue
		}
		addUser := false
		//TODO manage priviledges
		for _, gr := range usr.AccessGroups {
			if _, ok := groups[gr]; ok {
				addUser = true
				break
			}
		}
		if addUser {
			users[usr.UserHash] = *usr
		}
	}
	return users
}

//SetUsersDump drop table before adding users
func SetUsersDump(db Database, users map[string]duser.UserAccess) error {
	err := db.DropTable(pconst.DbConfig, AccessTable)
	if err != nil {
		return err
	}
	err = db.CreateTable(pconst.DbConfig, AccessTable, &users)
	if err != nil {
		return err
	}
	var res error
	for _, user := range users {
		_, err = db.InsertRecord(pconst.DbConfig, AccessTable, user)
		if err != nil {
			//best effort
			res = err
		}
	}
	return res
}
