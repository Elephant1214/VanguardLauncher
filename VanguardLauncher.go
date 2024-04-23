package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	args := os.Args
	if len(args) != 3 {
		fmt.Println("Usage: <fortnitePath> <username>")
		return
	}

	fnPath := args[1]
	username := args[2]
	if len(username) < 3 || len(username) > 16 {
		fmt.Println("Username must be 3-16 characters")
		return
	}

	suspend(fnPath + "\\FortniteGame\\Binaries\\Win64\\FortniteLauncher.exe")
	suspend(fnPath + "\\FortniteGame\\Binaries\\Win64\\FortniteClient-Win64-Shipping_EAC.exe")
	launchFN(fnPath, username)
}

func launchFN(path string, username string) {
	username = username + "@vanguard.dev"
	cmd := exec.Command(
		path+"\\FortniteGame\\Binaries\\Win64\\FortniteClient-Win64-Shipping.exe",
		"-epicapp=Fortnite",
		"-epicenv=Prod",
		"-epiclocale=en-us",
		"-epicportal",
		"-skippatchcheck",
		"-nobe",
		"-fromfl=eac",
		"-fltoken=3db3ba5dcbd2e16703f3978d",
		"-caldera=eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJhY2NvdW50X2lkIjoiYmU5ZGE1YzJmYmVhNDQwN2IyZjQwZWJhYWQ4NTlhZDQiLCJnZW5lcmF0ZWQiOjE2Mzg3MTcyNzgsImNhbGRlcmFHdWlkIjoiMzgxMGI4NjMtMmE2NS00NDU3LTliNTgtNGRhYjNiNDgyYTg2IiwiYWNQcm92aWRlciI6IkVhc3lBbnRpQ2hlYXQiLCJub3RlcyI6IiIsImZhbGxiYWNrIjpmYWxzZX0.VAWQB67RTxhiWOxx7DBjnzDnXyyEnX7OljJm-j2d88G_WgwQ9wrE6lwMEHZHjBd1ISJdUO1UVUqkfLdU5nofBQ",
		"-AUTH_LOGIN="+username,
		"-AUTH_PASSWORD=Vanguard",
		"-AUTH_TYPE=epic",
	)
	err := cmd.Start()
	if err != nil {
		log.Fatalln("Unable to start Fortnite:\n", err)
	}

	injectCobalt(uint32(cmd.Process.Pid), cmd)

	err = cmd.Wait()
	if err != nil {
		log.Println("Fortnite closed with a non-zero exit code:\n", err)
	}
}

func injectCobalt(pid uint32, cmd *exec.Cmd) {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	cobaltPath := filepath.Join(workingDir, "Cobalt.dll")

	_, err = os.Stat(cobaltPath)

	if err != nil && os.IsNotExist(err) {
		shutdownFortnite(cmd)
		log.Fatalln("Cobalt.dll was not found in the working directory:\n", err)
	} else if err != nil {
		shutdownFortnite(cmd)
		log.Fatalln("Error checking Cobalt.dll:\n", err)
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	handle, err := windows.OpenProcess(
		windows.PROCESS_CREATE_THREAD|windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_OPERATION|windows.PROCESS_VM_WRITE|windows.PROCESS_VM_READ,
		false,
		pid,
	)
	if err != nil {
		log.Fatalln("Unable to open Fortnite process:\n", err)
	}

	VirtualAllocEx := kernel32.NewProc("VirtualAllocEx")
	alloc, _, err := VirtualAllocEx.Call(
		uintptr(handle),
		0,
		uintptr(len(cobaltPath)+1),
		windows.MEM_RESERVE|windows.MEM_COMMIT, windows.PAGE_EXECUTE_READWRITE)
	bPtr, err := windows.BytePtrFromString(cobaltPath)
	if err != nil {
		shutdownFortnite(cmd)
		log.Fatalln("Unable to allocate memory for Cobalt:\n", err)
	}

	zero := uintptr(0)
	err = windows.WriteProcessMemory(handle, alloc, bPtr, uintptr(len(cobaltPath)+1), &zero)
	if err != nil {
		shutdownFortnite(cmd)
		log.Fatalln("Unable to write Fortnite process memory:\n", err)
	}

	LoadLibAddr, err := syscall.GetProcAddress(syscall.Handle(kernel32.Handle()), "LoadLibraryA")
	if err != nil {
		shutdownFortnite(cmd)
		log.Fatal("Unable to load Cobalt into memory:\n", err)
	}

	tHandle, _, _ := kernel32.NewProc("CreateRemoteThread").Call(uintptr(handle), 0, 0, LoadLibAddr, alloc, 0, 0)
	defer syscall.CloseHandle(syscall.Handle(tHandle))
}

func shutdownFortnite(cmd *exec.Cmd) {
	err := cmd.Process.Kill()
	if err != nil {
		log.Println("Unable to kill Fortnite process! Is it running?\n", err)
	}
}

func suspend(path string) {
	cmd := exec.Command(path)
	err := cmd.Start()
	if err != nil {
		log.Fatalln("Unable to start "+path+":\n", err)
	}

	handle, err := syscall.OpenProcess(windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if err != nil {
		log.Fatalln("Unable to open process:\n", err)
	}
	defer syscall.CloseHandle(handle)

	ntDll := syscall.NewLazyDLL("ntdll.dll")
	ntSuspendProcess := ntDll.NewProc("NtSuspendProcess")

	result, _, _ := ntSuspendProcess.Call(uintptr(handle))
	if result != 0 {
		log.Fatalf("Unable to suspend process %d\n", cmd.Process.Pid)
	}
}
