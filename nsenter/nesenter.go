package nsenter

/*
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>  // for open
#include <unistd.h> // for close

int setns(int fd, int nstype);

#define _GNU_SOURCE
#include <sched.h>



__attribute__((constructor)) void enter_namespace(void) {
	char *mydocker_pid;
	mydocker_pid = getenv("mydocker_pid");
	if (mydocker_pid) {
		fprintf(stdout, "INFO mydocker_pid = %s\n", mydocker_pid);
	} else {
		//fprintf(stdout, "INFO Missing mydocker_pid env skip nsenter\n");
		return;
	}
	char *mydocker_cmd;
	mydocker_cmd = getenv("mydocker_cmd");
	if (mydocker_cmd) {
		fprintf(stdout, "INFO mydocker_cmd = %s\n", mydocker_cmd);
	} else {
		//fprintf(stdout, "INFO Missing mydocker_cmd env skip nsenter\n");
		return;
	}

	// int setns(int fd, int nstype);
	//
	// Given a file descriptor referring to a namespace, reassociate the
	// calling thread with that namespace.
	//
	// The fd argument is a file descriptor referring to one of the
	// namespace entries in a /proc/[pid]/ns/ directory; see namespaces(7)
	// for further information on /proc/[pid]/ns/.  The calling thread will
	// be reassociated with the corresponding namespace, subject to any
	// constraints imposed by the nstype argument.

	int i;
	char nspath[1024];
	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };

	for (i=0; i<5; i++) {
		sprintf(nspath, "/proc/%s/ns/%s", mydocker_pid, namespaces[i]);
		int fd = open(nspath, O_RDONLY);

		if (setns(fd, 0) == -1) {
			fprintf(stderr, "ERRO setns on %s namespace failed: %s\n", namespaces[i], strerror(errno));
		} else {
			fprintf(stdout, "INFO setns on %s\n", namespaces[i]);
		}
		close(fd);
	}

	// The system() library function uses fork(2) to create a child process
	// that executes the shell command specified in command using execl(3)
	// as follows:
	//
	//     execl("/bin/sh", "sh", "-c", command, (char *) 0);
	//
	// system() returns after the command has been completed.
	//
	// During execution of the command, SIGCHLD will be blocked, and SIGINT
	// and SIGQUIT will be ignored, in the process that calls system()
	// (these signals will be handled according to their defaults inside the
	// child process that executes command).
	//
	// If command is NULL, then system() returns a status indicating whether
	// a shell is available on the system.
	int res = system(mydocker_cmd);
	fprintf(stdout, "fork(2); and exec /proc/self/exe -> %s", mydocker_cmd);

	exit(0);
	return;
}
*/
import "C"
