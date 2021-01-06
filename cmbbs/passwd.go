package cmbbs

import "C"
import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"unsafe"

	"github.com/Ptt-official-app/go-pttbbs/cache"
	"github.com/Ptt-official-app/go-pttbbs/crypt"
	"github.com/Ptt-official-app/go-pttbbs/ptttype"
	"github.com/Ptt-official-app/go-pttbbs/sem"
)

//GenPasswd
//
//If passwd as empty: return empty passwd (unable to login)
func GenPasswd(passwd []byte) (passwdHash *ptttype.Passwd_t, err error) {
	if passwd[0] == 0 {
		return &ptttype.Passwd_t{}, nil
	}

	num := rand.Intn(65536)
	saltc := [2]byte{
		byte(num & 0x7f),
		byte((num >> 8) & 0x7f),
	}

	result, err := crypt.Fcrypt(passwd, saltc[:])
	passwdHash = &ptttype.Passwd_t{}
	copy(passwdHash[:], result[:])
	return passwdHash, err
}

//CheckPasswd
//Params
//	expected: expected-passwd-hash
//	input: input-passwd
//
//Return
//	bool: true: good (password matched). false: bad (password not matched).
//	error: err
func CheckPasswd(expected []byte, input []byte) (bool, error) {
	pw, err := crypt.Fcrypt(input, expected)
	if err != nil {
		return false, err
	}
	return bytes.Equal(pw, expected), nil //requires the passwd-hash be exact match.
}

func LogAttempt(userID *ptttype.UserID_t, ip *ptttype.IPv4_t, isWithUserHome bool) {
}

//PasswdLoadUser
//Params
//	userID: user-id
//
//Return
//	Uid: uid
//	*ptttype.UserecRaw: user.
//	error: err.
func PasswdLoadUser(userID *ptttype.UserID_t) (ptttype.Uid, *ptttype.UserecRaw, error) {
	if userID == nil || userID[0] == 0 {
		return 0, nil, ptttype.ErrInvalidUserID
	}

	uid, err := cache.SearchUserRaw(userID, nil)
	if err != nil {
		return 0, nil, err
	}

	if !uid.IsValid() {
		return 0, nil, ptttype.ErrInvalidUserID
	}

	user, err := PasswdQuery(uid)
	if err != nil {
		return 0, nil, err
	}

	return uid, user, nil
}

//PasswdQuery
//Params
//	uid: uid
//
//Return
//	*ptttype.UserecRaw: user.
//	error: err.
func PasswdQuery(uid ptttype.Uid) (*ptttype.UserecRaw, error) {
	if !uid.IsValid() {
		return nil, ptttype.ErrInvalidUserID
	}

	file, err := os.Open(ptttype.FN_PASSWD)
	if err != nil {
		return nil, err
	}

	user := &ptttype.UserecRaw{}
	uidInFile := uid.ToUidInStore()
	offset := int64(ptttype.USEREC_RAW_SZ) * int64(uidInFile)
	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, err
	}
	err = binary.Read(file, binary.LittleEndian, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

//PasswdQueryPasswd
//Params
//	uid: uid
//
//Return
//	*ptttype.UserecRaw: user.
//	error: err.
func PasswdQueryPasswd(uid ptttype.Uid) (passwdHash *ptttype.Passwd_t, err error) {
	if !uid.IsValid() {
		return nil, ptttype.ErrInvalidUserID
	}

	file, err := os.Open(ptttype.FN_PASSWD)
	if err != nil {
		return nil, err
	}

	passwdHash = &ptttype.Passwd_t{}
	uidInFile := uid.ToUidInStore()
	offset := int64(ptttype.USEREC_RAW_SZ)*int64(uidInFile) + int64(unsafe.Offsetof(ptttype.USEREC_RAW.PasswdHash))
	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, err
	}
	err = binary.Read(file, binary.LittleEndian, passwdHash)
	if err != nil {
		return nil, err
	}

	return passwdHash, nil
}

func PasswdUpdate(uid ptttype.Uid, user *ptttype.UserecRaw) error {
	if !uid.IsValid() {
		return cache.ErrInvalidUID
	}

	file, err := os.OpenFile(ptttype.FN_PASSWD, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	uidInFile := uid.ToUidInStore()
	_, err = file.Seek(int64(ptttype.USEREC_RAW_SZ)*int64(uidInFile), 0)
	if err != nil {
		return err
	}

	err = binary.Write(file, binary.LittleEndian, user)
	if err != nil {
		return err
	}

	return nil
}

func PasswdUpdatePasswd(uid ptttype.Uid, passwdHash *ptttype.Passwd_t) error {
	if !uid.IsValid() {
		return cache.ErrInvalidUID
	}

	file, err := os.OpenFile(ptttype.FN_PASSWD, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	uidInFile := uid.ToUidInStore()
	offset := int64(ptttype.USEREC_RAW_SZ)*int64(uidInFile) + int64(unsafe.Offsetof(ptttype.USEREC_RAW.PasswdHash))
	_, err = file.Seek(offset, 0)
	if err != nil {
		return err
	}

	err = binary.Write(file, binary.LittleEndian, passwdHash)
	if err != nil {
		return err
	}

	return nil
}

func PasswdInit() error {
	if Sem != nil {
		return nil
	}

	var err error
	Sem, err = sem.SemGet(ptttype.PASSWDSEM_KEY, 1, sem.SEM_R|sem.SEM_A|sem.IPC_CREAT|sem.IPC_EXCL)
	if err != nil {
		if os.IsExist(err) {
			Sem, err = sem.SemGet(ptttype.PASSWDSEM_KEY, 1, sem.SEM_R|sem.SEM_A)
			if err != nil {
				return err
			}

			return nil
		} else {
			return err
		}
	}

	err = Sem.SetVal(0, 1)
	if err != nil {
		return err
	}

	_, err = Sem.GetVal(0)
	if err != nil {
		return err
	}

	return nil
}

func PasswdLock() error {
	return Sem.Wait(0)
}

func PasswdUnlock() error {
	return Sem.Post(0)
}

//PasswdDestroy
//
//XXX [WARNING] know what you are doing before using Close!.
//This is to be able to close the semaphore for the completeness of the sem-usage.
//However, in production, we create sem without the need of closing the sem.
//
//We simply use ipcrm to delete the sem if necessary.
//
//Currently used only in test.
//
//XXX [2020-12-06] We don't do PasswdDestroy.
//                 Just let PasswdInit do the checking to avoid
//                 the duplication of sem.
func PasswdDestroy() error {
	return nil

	/*
		if !IsTest {
			return ErrInvalidOp
		}

		if Sem == nil {
			return ErrSemNotExists
		}

		err := Sem.Destroy(0)
		if err != nil {
			log.Errorf("cmbbs.PasswdDestroy: unable to close: e: %v", err)
			return err
		}

		Sem = nil

		log.Infof("cmbbs.PasswdDestroy: done")

		return nil
	*/
}
