// .git/HEAD
ref: refs/heads/main


// .git/config
[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = git@github.com:diki-haryadi/go-oauth2-server.git
	fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
	remote = origin
	merge = refs/heads/main


// .git/description
Unnamed repository; edit this file 'description' to name the repository.


// .git/hooks/applypatch-msg.sample
#!/bin/sh
#
# An example hook script to check the commit log message taken by
# applypatch from an e-mail message.
#
# The hook should exit with non-zero status after issuing an
# appropriate message if it wants to stop the commit.  The hook is
# allowed to edit the commit message file.
#
# To enable this hook, rename this file to "applypatch-msg".

. git-sh-setup
commitmsg="$(git rev-parse --git-path hooks/commit-msg)"
test -x "$commitmsg" && exec "$commitmsg" ${1+"$@"}
:


// .git/hooks/commit-msg.sample
#!/bin/sh
#
# An example hook script to check the commit log message.
# Called by "git commit" with one argument, the name of the file
# that has the commit message.  The hook should exit with non-zero
# status after issuing an appropriate message if it wants to stop the
# commit.  The hook is allowed to edit the commit message file.
#
# To enable this hook, rename this file to "commit-msg".

# Uncomment the below to add a Signed-off-by line to the message.
# Doing this in a hook is a bad idea in general, but the prepare-commit-msg
# hook is more suited to it.
#
# SOB=$(git var GIT_AUTHOR_IDENT | sed -n 's/^\(.*>\).*$/Signed-off-by: \1/p')
# grep -qs "^$SOB" "$1" || echo "$SOB" >> "$1"

# This example catches duplicate Signed-off-by lines.

test "" = "$(grep '^Signed-off-by: ' "$1" |
	 sort | uniq -c | sed -e '/^[ 	]*1[ 	]/d')" || {
	echo >&2 Duplicate Signed-off-by lines.
	exit 1
}


// .git/hooks/fsmonitor-watchman.sample
#!/usr/bin/perl

use strict;
use warnings;
use IPC::Open2;

# An example hook script to integrate Watchman
# (https://facebook.github.io/watchman/) with git to speed up detecting
# new and modified files.
#
# The hook is passed a version (currently 2) and last update token
# formatted as a string and outputs to stdout a new update token and
# all files that have been modified since the update token. Paths must
# be relative to the root of the working tree and separated by a single NUL.
#
# To enable this hook, rename this file to "query-watchman" and set
# 'git config core.fsmonitor .git/hooks/query-watchman'
#
my ($version, $last_update_token) = @ARGV;

# Uncomment for debugging
# print STDERR "$0 $version $last_update_token\n";

# Check the hook interface version
if ($version ne 2) {
	die "Unsupported query-fsmonitor hook version '$version'.\n" .
	    "Falling back to scanning...\n";
}

my $git_work_tree = get_working_dir();

my $retry = 1;

my $json_pkg;
eval {
	require JSON::XS;
	$json_pkg = "JSON::XS";
	1;
} or do {
	require JSON::PP;
	$json_pkg = "JSON::PP";
};

launch_watchman();

sub launch_watchman {
	my $o = watchman_query();
	if (is_work_tree_watched($o)) {
		output_result($o->{clock}, @{$o->{files}});
	}
}

sub output_result {
	my ($clockid, @files) = @_;

	# Uncomment for debugging watchman output
	# open (my $fh, ">", ".git/watchman-output.out");
	# binmode $fh, ":utf8";
	# print $fh "$clockid\n@files\n";
	# close $fh;

	binmode STDOUT, ":utf8";
	print $clockid;
	print "\0";
	local $, = "\0";
	print @files;
}

sub watchman_clock {
	my $response = qx/watchman clock "$git_work_tree"/;
	die "Failed to get clock id on '$git_work_tree'.\n" .
		"Falling back to scanning...\n" if $? != 0;

	return $json_pkg->new->utf8->decode($response);
}

sub watchman_query {
	my $pid = open2(\*CHLD_OUT, \*CHLD_IN, 'watchman -j --no-pretty')
	or die "open2() failed: $!\n" .
	"Falling back to scanning...\n";

	# In the query expression below we're asking for names of files that
	# changed since $last_update_token but not from the .git folder.
	#
	# To accomplish this, we're using the "since" generator to use the
	# recency index to select candidate nodes and "fields" to limit the
	# output to file names only. Then we're using the "expression" term to
	# further constrain the results.
	my $last_update_line = "";
	if (substr($last_update_token, 0, 1) eq "c") {
		$last_update_token = "\"$last_update_token\"";
		$last_update_line = qq[\n"since": $last_update_token,];
	}
	my $query = <<"	END";
		["query", "$git_work_tree", {$last_update_line
			"fields": ["name"],
			"expression": ["not", ["dirname", ".git"]]
		}]
	END

	# Uncomment for debugging the watchman query
	# open (my $fh, ">", ".git/watchman-query.json");
	# print $fh $query;
	# close $fh;

	print CHLD_IN $query;
	close CHLD_IN;
	my $response = do {local $/; <CHLD_OUT>};

	# Uncomment for debugging the watch response
	# open ($fh, ">", ".git/watchman-response.json");
	# print $fh $response;
	# close $fh;

	die "Watchman: command returned no output.\n" .
	"Falling back to scanning...\n" if $response eq "";
	die "Watchman: command returned invalid output: $response\n" .
	"Falling back to scanning...\n" unless $response =~ /^\{/;

	return $json_pkg->new->utf8->decode($response);
}

sub is_work_tree_watched {
	my ($output) = @_;
	my $error = $output->{error};
	if ($retry > 0 and $error and $error =~ m/unable to resolve root .* directory (.*) is not watched/) {
		$retry--;
		my $response = qx/watchman watch "$git_work_tree"/;
		die "Failed to make watchman watch '$git_work_tree'.\n" .
		    "Falling back to scanning...\n" if $? != 0;
		$output = $json_pkg->new->utf8->decode($response);
		$error = $output->{error};
		die "Watchman: $error.\n" .
		"Falling back to scanning...\n" if $error;

		# Uncomment for debugging watchman output
		# open (my $fh, ">", ".git/watchman-output.out");
		# close $fh;

		# Watchman will always return all files on the first query so
		# return the fast "everything is dirty" flag to git and do the
		# Watchman query just to get it over with now so we won't pay
		# the cost in git to look up each individual file.
		my $o = watchman_clock();
		$error = $output->{error};

		die "Watchman: $error.\n" .
		"Falling back to scanning...\n" if $error;

		output_result($o->{clock}, ("/"));
		$last_update_token = $o->{clock};

		eval { launch_watchman() };
		return 0;
	}

	die "Watchman: $error.\n" .
	"Falling back to scanning...\n" if $error;

	return 1;
}

sub get_working_dir {
	my $working_dir;
	if ($^O =~ 'msys' || $^O =~ 'cygwin') {
		$working_dir = Win32::GetCwd();
		$working_dir =~ tr/\\/\//;
	} else {
		require Cwd;
		$working_dir = Cwd::cwd();
	}

	return $working_dir;
}


// .git/hooks/post-update.sample
#!/bin/sh
#
# An example hook script to prepare a packed repository for use over
# dumb transports.
#
# To enable this hook, rename this file to "post-update".

exec git update-server-info


// .git/hooks/pre-applypatch.sample
#!/bin/sh
#
# An example hook script to verify what is about to be committed
# by applypatch from an e-mail message.
#
# The hook should exit with non-zero status after issuing an
# appropriate message if it wants to stop the commit.
#
# To enable this hook, rename this file to "pre-applypatch".

. git-sh-setup
precommit="$(git rev-parse --git-path hooks/pre-commit)"
test -x "$precommit" && exec "$precommit" ${1+"$@"}
:


// .git/hooks/pre-commit.sample
#!/bin/sh
#
# An example hook script to verify what is about to be committed.
# Called by "git commit" with no arguments.  The hook should
# exit with non-zero status after issuing an appropriate message if
# it wants to stop the commit.
#
# To enable this hook, rename this file to "pre-commit".

if git rev-parse --verify HEAD >/dev/null 2>&1
then
	against=HEAD
else
	# Initial commit: diff against an empty tree object
	against=$(git hash-object -t tree /dev/null)
fi

# If you want to allow non-ASCII filenames set this variable to true.
allownonascii=$(git config --type=bool hooks.allownonascii)

# Redirect output to stderr.
exec 1>&2

# Cross platform projects tend to avoid non-ASCII filenames; prevent
# them from being added to the repository. We exploit the fact that the
# printable range starts at the space character and ends with tilde.
if [ "$allownonascii" != "true" ] &&
	# Note that the use of brackets around a tr range is ok here, (it's
	# even required, for portability to Solaris 10's /usr/bin/tr), since
	# the square bracket bytes happen to fall in the designated range.
	test $(git diff-index --cached --name-only --diff-filter=A -z $against |
	  LC_ALL=C tr -d '[ -~]\0' | wc -c) != 0
then
	cat <<\EOF
Error: Attempt to add a non-ASCII file name.

This can cause problems if you want to work with people on other platforms.

To be portable it is advisable to rename the file.

If you know what you are doing you can disable this check using:

  git config hooks.allownonascii true
EOF
	exit 1
fi

# If there are whitespace errors, print the offending file names and fail.
exec git diff-index --check --cached $against --


// .git/hooks/pre-merge-commit.sample
#!/bin/sh
#
# An example hook script to verify what is about to be committed.
# Called by "git merge" with no arguments.  The hook should
# exit with non-zero status after issuing an appropriate message to
# stderr if it wants to stop the merge commit.
#
# To enable this hook, rename this file to "pre-merge-commit".

. git-sh-setup
test -x "$GIT_DIR/hooks/pre-commit" &&
        exec "$GIT_DIR/hooks/pre-commit"
:


// .git/hooks/pre-push.sample
#!/bin/sh

# An example hook script to verify what is about to be pushed.  Called by "git
# push" after it has checked the remote status, but before anything has been
# pushed.  If this script exits with a non-zero status nothing will be pushed.
#
# This hook is called with the following parameters:
#
# $1 -- Name of the remote to which the push is being done
# $2 -- URL to which the push is being done
#
# If pushing without using a named remote those arguments will be equal.
#
# Information about the commits which are being pushed is supplied as lines to
# the standard input in the form:
#
#   <local ref> <local oid> <remote ref> <remote oid>
#
# This sample shows how to prevent push of commits where the log message starts
# with "WIP" (work in progress).

remote="$1"
url="$2"

zero=$(git hash-object --stdin </dev/null | tr '[0-9a-f]' '0')

while read local_ref local_oid remote_ref remote_oid
do
	if test "$local_oid" = "$zero"
	then
		# Handle delete
		:
	else
		if test "$remote_oid" = "$zero"
		then
			# New branch, examine all commits
			range="$local_oid"
		else
			# Update to existing branch, examine new commits
			range="$remote_oid..$local_oid"
		fi

		# Check for WIP commit
		commit=$(git rev-list -n 1 --grep '^WIP' "$range")
		if test -n "$commit"
		then
			echo >&2 "Found WIP commit in $local_ref, not pushing"
			exit 1
		fi
	fi
done

exit 0


// .git/hooks/pre-rebase.sample
#!/bin/sh
#
# Copyright (c) 2006, 2008 Junio C Hamano
#
# The "pre-rebase" hook is run just before "git rebase" starts doing
# its job, and can prevent the command from running by exiting with
# non-zero status.
#
# The hook is called with the following parameters:
#
# $1 -- the upstream the series was forked from.
# $2 -- the branch being rebased (or empty when rebasing the current branch).
#
# This sample shows how to prevent topic branches that are already
# merged to 'next' branch from getting rebased, because allowing it
# would result in rebasing already published history.

publish=next
basebranch="$1"
if test "$#" = 2
then
	topic="refs/heads/$2"
else
	topic=`git symbolic-ref HEAD` ||
	exit 0 ;# we do not interrupt rebasing detached HEAD
fi

case "$topic" in
refs/heads/??/*)
	;;
*)
	exit 0 ;# we do not interrupt others.
	;;
esac

# Now we are dealing with a topic branch being rebased
# on top of master.  Is it OK to rebase it?

# Does the topic really exist?
git show-ref -q "$topic" || {
	echo >&2 "No such branch $topic"
	exit 1
}

# Is topic fully merged to master?
not_in_master=`git rev-list --pretty=oneline ^master "$topic"`
if test -z "$not_in_master"
then
	echo >&2 "$topic is fully merged to master; better remove it."
	exit 1 ;# we could allow it, but there is no point.
fi

# Is topic ever merged to next?  If so you should not be rebasing it.
only_next_1=`git rev-list ^master "^$topic" ${publish} | sort`
only_next_2=`git rev-list ^master           ${publish} | sort`
if test "$only_next_1" = "$only_next_2"
then
	not_in_topic=`git rev-list "^$topic" master`
	if test -z "$not_in_topic"
	then
		echo >&2 "$topic is already up to date with master"
		exit 1 ;# we could allow it, but there is no point.
	else
		exit 0
	fi
else
	not_in_next=`git rev-list --pretty=oneline ^${publish} "$topic"`
	/usr/bin/perl -e '
		my $topic = $ARGV[0];
		my $msg = "* $topic has commits already merged to public branch:\n";
		my (%not_in_next) = map {
			/^([0-9a-f]+) /;
			($1 => 1);
		} split(/\n/, $ARGV[1]);
		for my $elem (map {
				/^([0-9a-f]+) (.*)$/;
				[$1 => $2];
			} split(/\n/, $ARGV[2])) {
			if (!exists $not_in_next{$elem->[0]}) {
				if ($msg) {
					print STDERR $msg;
					undef $msg;
				}
				print STDERR " $elem->[1]\n";
			}
		}
	' "$topic" "$not_in_next" "$not_in_master"
	exit 1
fi

<<\DOC_END

This sample hook safeguards topic branches that have been
published from being rewound.

The workflow assumed here is:

 * Once a topic branch forks from "master", "master" is never
   merged into it again (either directly or indirectly).

 * Once a topic branch is fully cooked and merged into "master",
   it is deleted.  If you need to build on top of it to correct
   earlier mistakes, a new topic branch is created by forking at
   the tip of the "master".  This is not strictly necessary, but
   it makes it easier to keep your history simple.

 * Whenever you need to test or publish your changes to topic
   branches, merge them into "next" branch.

The script, being an example, hardcodes the publish branch name
to be "next", but it is trivial to make it configurable via
$GIT_DIR/config mechanism.

With this workflow, you would want to know:

(1) ... if a topic branch has ever been merged to "next".  Young
    topic branches can have stupid mistakes you would rather
    clean up before publishing, and things that have not been
    merged into other branches can be easily rebased without
    affecting other people.  But once it is published, you would
    not want to rewind it.

(2) ... if a topic branch has been fully merged to "master".
    Then you can delete it.  More importantly, you should not
    build on top of it -- other people may already want to
    change things related to the topic as patches against your
    "master", so if you need further changes, it is better to
    fork the topic (perhaps with the same name) afresh from the
    tip of "master".

Let's look at this example:

		   o---o---o---o---o---o---o---o---o---o "next"
		  /       /           /           /
		 /   a---a---b A     /           /
		/   /               /           /
	       /   /   c---c---c---c B         /
	      /   /   /             \         /
	     /   /   /   b---b C     \       /
	    /   /   /   /             \     /
    ---o---o---o---o---o---o---o---o---o---o---o "master"


A, B and C are topic branches.

 * A has one fix since it was merged up to "next".

 * B has finished.  It has been fully merged up to "master" and "next",
   and is ready to be deleted.

 * C has not merged to "next" at all.

We would want to allow C to be rebased, refuse A, and encourage
B to be deleted.

To compute (1):

	git rev-list ^master ^topic next
	git rev-list ^master        next

	if these match, topic has not merged in next at all.

To compute (2):

	git rev-list master..topic

	if this is empty, it is fully merged to "master".

DOC_END


// .git/hooks/pre-receive.sample
#!/bin/sh
#
# An example hook script to make use of push options.
# The example simply echoes all push options that start with 'echoback='
# and rejects all pushes when the "reject" push option is used.
#
# To enable this hook, rename this file to "pre-receive".

if test -n "$GIT_PUSH_OPTION_COUNT"
then
	i=0
	while test "$i" -lt "$GIT_PUSH_OPTION_COUNT"
	do
		eval "value=\$GIT_PUSH_OPTION_$i"
		case "$value" in
		echoback=*)
			echo "echo from the pre-receive-hook: ${value#*=}" >&2
			;;
		reject)
			exit 1
		esac
		i=$((i + 1))
	done
fi


// .git/hooks/prepare-commit-msg.sample
#!/bin/sh
#
# An example hook script to prepare the commit log message.
# Called by "git commit" with the name of the file that has the
# commit message, followed by the description of the commit
# message's source.  The hook's purpose is to edit the commit
# message file.  If the hook fails with a non-zero status,
# the commit is aborted.
#
# To enable this hook, rename this file to "prepare-commit-msg".

# This hook includes three examples. The first one removes the
# "# Please enter the commit message..." help message.
#
# The second includes the output of "git diff --name-status -r"
# into the message, just before the "git status" output.  It is
# commented because it doesn't cope with --amend or with squashed
# commits.
#
# The third example adds a Signed-off-by line to the message, that can
# still be edited.  This is rarely a good idea.

COMMIT_MSG_FILE=$1
COMMIT_SOURCE=$2
SHA1=$3

/usr/bin/perl -i.bak -ne 'print unless(m/^. Please enter the commit message/..m/^#$/)' "$COMMIT_MSG_FILE"

# case "$COMMIT_SOURCE,$SHA1" in
#  ,|template,)
#    /usr/bin/perl -i.bak -pe '
#       print "\n" . `git diff --cached --name-status -r`
# 	 if /^#/ && $first++ == 0' "$COMMIT_MSG_FILE" ;;
#  *) ;;
# esac

# SOB=$(git var GIT_COMMITTER_IDENT | sed -n 's/^\(.*>\).*$/Signed-off-by: \1/p')
# git interpret-trailers --in-place --trailer "$SOB" "$COMMIT_MSG_FILE"
# if test -z "$COMMIT_SOURCE"
# then
#   /usr/bin/perl -i.bak -pe 'print "\n" if !$first_line++' "$COMMIT_MSG_FILE"
# fi


// .git/hooks/push-to-checkout.sample
#!/bin/sh

# An example hook script to update a checked-out tree on a git push.
#
# This hook is invoked by git-receive-pack(1) when it reacts to git
# push and updates reference(s) in its repository, and when the push
# tries to update the branch that is currently checked out and the
# receive.denyCurrentBranch configuration variable is set to
# updateInstead.
#
# By default, such a push is refused if the working tree and the index
# of the remote repository has any difference from the currently
# checked out commit; when both the working tree and the index match
# the current commit, they are updated to match the newly pushed tip
# of the branch. This hook is to be used to override the default
# behaviour; however the code below reimplements the default behaviour
# as a starting point for convenient modification.
#
# The hook receives the commit with which the tip of the current
# branch is going to be updated:
commit=$1

# It can exit with a non-zero status to refuse the push (when it does
# so, it must not modify the index or the working tree).
die () {
	echo >&2 "$*"
	exit 1
}

# Or it can make any necessary changes to the working tree and to the
# index to bring them to the desired state when the tip of the current
# branch is updated to the new commit, and exit with a zero status.
#
# For example, the hook can simply run git read-tree -u -m HEAD "$1"
# in order to emulate git fetch that is run in the reverse direction
# with git push, as the two-tree form of git read-tree -u -m is
# essentially the same as git switch or git checkout that switches
# branches while keeping the local changes in the working tree that do
# not interfere with the difference between the branches.

# The below is a more-or-less exact translation to shell of the C code
# for the default behaviour for git's push-to-checkout hook defined in
# the push_to_deploy() function in builtin/receive-pack.c.
#
# Note that the hook will be executed from the repository directory,
# not from the working tree, so if you want to perform operations on
# the working tree, you will have to adapt your code accordingly, e.g.
# by adding "cd .." or using relative paths.

if ! git update-index -q --ignore-submodules --refresh
then
	die "Up-to-date check failed"
fi

if ! git diff-files --quiet --ignore-submodules --
then
	die "Working directory has unstaged changes"
fi

# This is a rough translation of:
#
#   head_has_history() ? "HEAD" : EMPTY_TREE_SHA1_HEX
if git cat-file -e HEAD 2>/dev/null
then
	head=HEAD
else
	head=$(git hash-object -t tree --stdin </dev/null)
fi

if ! git diff-index --quiet --cached --ignore-submodules $head --
then
	die "Working directory has staged changes"
fi

if ! git read-tree -u -m "$commit"
then
	die "Could not update working tree to new HEAD"
fi


// .git/hooks/sendemail-validate.sample
#!/bin/sh

# An example hook script to validate a patch (and/or patch series) before
# sending it via email.
#
# The hook should exit with non-zero status after issuing an appropriate
# message if it wants to prevent the email(s) from being sent.
#
# To enable this hook, rename this file to "sendemail-validate".
#
# By default, it will only check that the patch(es) can be applied on top of
# the default upstream branch without conflicts in a secondary worktree. After
# validation (successful or not) of the last patch of a series, the worktree
# will be deleted.
#
# The following config variables can be set to change the default remote and
# remote ref that are used to apply the patches against:
#
#   sendemail.validateRemote (default: origin)
#   sendemail.validateRemoteRef (default: HEAD)
#
# Replace the TODO placeholders with appropriate checks according to your
# needs.

validate_cover_letter () {
	file="$1"
	# TODO: Replace with appropriate checks (e.g. spell checking).
	true
}

validate_patch () {
	file="$1"
	# Ensure that the patch applies without conflicts.
	git am -3 "$file" || return
	# TODO: Replace with appropriate checks for this patch
	# (e.g. checkpatch.pl).
	true
}

validate_series () {
	# TODO: Replace with appropriate checks for the whole series
	# (e.g. quick build, coding style checks, etc.).
	true
}

# main -------------------------------------------------------------------------

if test "$GIT_SENDEMAIL_FILE_COUNTER" = 1
then
	remote=$(git config --default origin --get sendemail.validateRemote) &&
	ref=$(git config --default HEAD --get sendemail.validateRemoteRef) &&
	worktree=$(mktemp --tmpdir -d sendemail-validate.XXXXXXX) &&
	git worktree add -fd --checkout "$worktree" "refs/remotes/$remote/$ref" &&
	git config --replace-all sendemail.validateWorktree "$worktree"
else
	worktree=$(git config --get sendemail.validateWorktree)
fi || {
	echo "sendemail-validate: error: failed to prepare worktree" >&2
	exit 1
}

unset GIT_DIR GIT_WORK_TREE
cd "$worktree" &&

if grep -q "^diff --git " "$1"
then
	validate_patch "$1"
else
	validate_cover_letter "$1"
fi &&

if test "$GIT_SENDEMAIL_FILE_COUNTER" = "$GIT_SENDEMAIL_FILE_TOTAL"
then
	git config --unset-all sendemail.validateWorktree &&
	trap 'git worktree remove -ff "$worktree"' EXIT &&
	validate_series
fi


// .git/hooks/update.sample
#!/bin/sh
#
# An example hook script to block unannotated tags from entering.
# Called by "git receive-pack" with arguments: refname sha1-old sha1-new
#
# To enable this hook, rename this file to "update".
#
# Config
# ------
# hooks.allowunannotated
#   This boolean sets whether unannotated tags will be allowed into the
#   repository.  By default they won't be.
# hooks.allowdeletetag
#   This boolean sets whether deleting tags will be allowed in the
#   repository.  By default they won't be.
# hooks.allowmodifytag
#   This boolean sets whether a tag may be modified after creation. By default
#   it won't be.
# hooks.allowdeletebranch
#   This boolean sets whether deleting branches will be allowed in the
#   repository.  By default they won't be.
# hooks.denycreatebranch
#   This boolean sets whether remotely creating branches will be denied
#   in the repository.  By default this is allowed.
#

# --- Command line
refname="$1"
oldrev="$2"
newrev="$3"

# --- Safety check
if [ -z "$GIT_DIR" ]; then
	echo "Don't run this script from the command line." >&2
	echo " (if you want, you could supply GIT_DIR then run" >&2
	echo "  $0 <ref> <oldrev> <newrev>)" >&2
	exit 1
fi

if [ -z "$refname" -o -z "$oldrev" -o -z "$newrev" ]; then
	echo "usage: $0 <ref> <oldrev> <newrev>" >&2
	exit 1
fi

# --- Config
allowunannotated=$(git config --type=bool hooks.allowunannotated)
allowdeletebranch=$(git config --type=bool hooks.allowdeletebranch)
denycreatebranch=$(git config --type=bool hooks.denycreatebranch)
allowdeletetag=$(git config --type=bool hooks.allowdeletetag)
allowmodifytag=$(git config --type=bool hooks.allowmodifytag)

# check for no description
projectdesc=$(sed -e '1q' "$GIT_DIR/description")
case "$projectdesc" in
"Unnamed repository"* | "")
	echo "*** Project description file hasn't been set" >&2
	exit 1
	;;
esac

# --- Check types
# if $newrev is 0000...0000, it's a commit to delete a ref.
zero=$(git hash-object --stdin </dev/null | tr '[0-9a-f]' '0')
if [ "$newrev" = "$zero" ]; then
	newrev_type=delete
else
	newrev_type=$(git cat-file -t $newrev)
fi

case "$refname","$newrev_type" in
	refs/tags/*,commit)
		# un-annotated tag
		short_refname=${refname##refs/tags/}
		if [ "$allowunannotated" != "true" ]; then
			echo "*** The un-annotated tag, $short_refname, is not allowed in this repository" >&2
			echo "*** Use 'git tag [ -a | -s ]' for tags you want to propagate." >&2
			exit 1
		fi
		;;
	refs/tags/*,delete)
		# delete tag
		if [ "$allowdeletetag" != "true" ]; then
			echo "*** Deleting a tag is not allowed in this repository" >&2
			exit 1
		fi
		;;
	refs/tags/*,tag)
		# annotated tag
		if [ "$allowmodifytag" != "true" ] && git rev-parse $refname > /dev/null 2>&1
		then
			echo "*** Tag '$refname' already exists." >&2
			echo "*** Modifying a tag is not allowed in this repository." >&2
			exit 1
		fi
		;;
	refs/heads/*,commit)
		# branch
		if [ "$oldrev" = "$zero" -a "$denycreatebranch" = "true" ]; then
			echo "*** Creating a branch is not allowed in this repository" >&2
			exit 1
		fi
		;;
	refs/heads/*,delete)
		# delete branch
		if [ "$allowdeletebranch" != "true" ]; then
			echo "*** Deleting a branch is not allowed in this repository" >&2
			exit 1
		fi
		;;
	refs/remotes/*,commit)
		# tracking branch
		;;
	refs/remotes/*,delete)
		# delete tracking branch
		if [ "$allowdeletebranch" != "true" ]; then
			echo "*** Deleting a tracking branch is not allowed in this repository" >&2
			exit 1
		fi
		;;
	*)
		# Anything else (is there anything else?)
		echo "*** Update hook: unknown type of update to ref $refname of type $newrev_type" >&2
		exit 1
		;;
esac

# --- Finished
exit 0


// .git/index
DIRC      �g�~�r��g�~�r��   ' �'  ��  �  �  M75鿁�����k���1���> 
.gitignore        g�~�r��g�~�r��   ' �(  ��  �  �  D��%�A�m�u��~8��� .pre-commit-config.yaml   g�~�r��g�~�r��   ' �)  ��  �  �  -`ժC_��d"~E�}�ʄp� LICENSE   g�~�r��g�~�r��   ' �*  ��  �  �  Z�ߢO�V�{� ��Z�zPS� Makefile  g�~�r��g�~�r��   ' �+  ��  �  �  �'c;���Al����n[j 	README.md g�~�r��g�~�r��   ' �-  ��  �  �  ����C ��$ʌ���S��{ 
app/app.go        g�~�r��g�~�r��   ' �/  ��  �  �  
�����n�Ɏ)�T�G^:�� cmd/load_data.go  g�~�r��g�~�r��   ' �0  ��  �  �  g�h�3��S �E����*( cmd/root.go       g�~�r��g�~�r��   ' �1  ��  �  �  �(��~.�cf�3�T��f�� cmd/serve.go      g�~�r��g�~�r��   ' �3  ��  �  �  �XNY�I���m���8�TӁ config/config.go  g�~�r��g�~�r��   ' �6  ��  �  �  ����Qo ��;&��b�>��G db/fixtures/roles.yml     g�~�r��g�~�r��   ' �7  ��  �  �  ���UU��H�5kNl��l��� db/fixtures/scopes.yml    g�~�r��g�~�r��   ' �8  ��  �  �  �]ӛCR��.3<�g������ "db/fixtures/test_access_tokens.yml        g�~��55g�~��55   ' �9  ��  �  �  �v�� ��h��7�����t�. db/fixtures/test_clients.yml      g�~��55g�~��55   ' �:  ��  �  �  ��غ{��cUV��edԠ���Y� db/fixtures/test_users.yml        g�~��55g�~��55   ' �<  ��  �  �    �⛲��CK�)�wZ���S� 2db/migrations/20221110221143_migrate_name.down.sql        g�~��55g�~��55   ' �=  ��  �  �   ���L�R�#�/T����2 0db/migrations/20221110221143_migrate_name.up.sql  g�~��55g�~��55   ' �>  ��  �  �   �d}!�m/�+�������b� +db/migrations/20240908110637_users.down.sql       g�~��55g�~��55   ' �?  ��  �  �  ��Z
��8�F>�f�~�e� )db/migrations/20240908110637_users.up.sql g�~��55g�~��55   ' �@  ��  �  �   -�8��eʸM>r�&�H� -db/migrations/20241003072848_clients.down.sql     g�~��55g�~��55   ' �A  ��  �  �  ݨ1�F����	�})洎�,� +db/migrations/20241003072848_clients.up.sql       g�~��55g�~��55   ' �B  ��  �  �   �Q�_A������g���� ,db/migrations/20241003072908_scopes.down.sql      g�~��55g�~��55   ' �C  ��  �  �  H�7��M1�[<�U�m��'� *db/migrations/20241003072908_scopes.up.sql        g�~��55g�~��55   ' �D  ��  �  �   ��-���*\��^��r�, +db/migrations/20241003072922_roles.down.sql       g�~��55g�~��55   ' �E  ��  �  �  C/�?���}�ݴ#M�~�ND� )db/migrations/20241003072922_roles.up.sql g�~��55g�~��55   ' �F  ��  �  �   >��Ix\h0!��IhO��� 4db/migrations/20241003072940_refresh_tokens.down.sql      g�~��55g�~��55   ' �G  ��  �  �  ��O���{��Ì~��u! 2db/migrations/20241003072940_refresh_tokens.up.sql        g�~��55g�~��55   ' �H  ��  �  �   �&@0�E�'�3ۡ��p5�� 3db/migrations/20241003072953_access_tokens.down.sql       g�~��55g�~��55   ' �I  ��  �  �  ���N��Yl��g%@t�t��I� 1db/migrations/20241003072953_access_tokens.up.sql g�~��55g�~��55   ' �J  ��  �  �    ��p�xHQK�=Eա��U�� 9db/migrations/20241003073005_authorization_codes.down.sql g�~��55g�~��55   ' �K  ��  �  �  ��4�5�Y�k���S'=���*� 7db/migrations/20241003073005_authorization_codes.up.sql   g�~��55g�~��55   ' �M  ��  �  �  K�O2ns
e�M-�	:FWss�� deployments/cassandra.yml g�~��55g�~��55   ' �N  ��  �  �  �X�����:�5ڵL>a6�ˁ )deployments/docker-compose.e2e-local.yaml g�~��55g�~��55   ' �O  ��  �  �  gD�z�V�C��s3 �qW�3� deployments/docker-compose.yaml   g�~��55g�~��55   ' �Q  ��  �  �  
�԰��<W��j����d'd2 docs/admin.http   g�~��55g�~��55   ' �S  ��  �  �  I��3��2,J�\�-�r�-�K� ,docs/api-specification/authorization_code.md      g�~��55g�~��55   ' �T  ��  �  �  ���H?\�+��I�����7 $docs/api-specification/introspect.md      g�~��55g�~��55   ' �U  ��  �  �   ���5Y N���`���>F +docs/api-specification/oauth_credentials.md       g�~��55g�~��55   ' �V  ��  �  �  IPS�B<u�"I�.�h�i "docs/api-specification/password.md        g�~��55g�~��55   ' �W  ��  �  �   ��`R���>�� ���Z�s� 'docs/api-specification/refresh_token.md   g�~��55g�~��55   ' �X  ��  �  �  ����s  �|s5$OY��ڬ�� $docs/common-oauth2-server-feature.md      g�~��55g�~��55   ' �Y  ��  �  �  ˣ�d��.���.����xq� 
docs/db.md        g�~��55g�~��55   ' �Z  ��  �  �  mw�~���
�]�����{ؗ�� docs/deployment.md        g�~��wug�~��wu   ' �[  ��  �  �  )�P���K���qΌ��A3+ docs/design.md    g�~��wug�~��wu   ' �\  ��  �  �   ���f���$T�eJO�,7�� docs/devops.md    g�~��wug�~��wu   ' �]  ��  �  �  �W�5�������I�� docs/env.md       g�~��wug�~��wu   ' �^  ��  �  �   ��<	8���k�-Q#��� docs/flow-diagram.md      g�~��wug�~��wu   ' �_  ��  �  �  6Q�h���n%"�����!b�� docs/oauth.http   g�~��wug�~��wu   ' �a  ��  �  �  qV����i���PI�趧� !docs/requirement/user-consents.md g�~��wug�~��wu   ' �c  ��  �  �  e�XK�Ń���1o�"�lQּ� envs/local.env    g�~��wug�~��wu   ' �d  ��  �  �  ah�kd$�U8��=�n���� envs/production.env       g�~��wug�~��wu   ' �e  ��  �  �  g�����6L.e${x^�f
 envs/stage.env    g�~��wug�~��wu   ' �f  ��  �  �  a\���H��y�Ị[�_� envs/test.env     g�~��wug�~��wu   ' �j  ��  �  �  *Xfӣ��׀��g�o�� �� ?external/sample_ext_service/domain/sample_ext_service_domain.go   g�~��wug�~��wu   ' �l  ��  �  �  ���l;���A!xtޏ�mZr� Aexternal/sample_ext_service/usecase/sample_ext_service_usecase.go g�~��wug�~��wu   ' �m  ��  �  �  ܆�X+g�0�?��[�RV go.mod    g�~��wug�~��wu   ' �n  ��  �  �  kV�vp��͠x�?j"��C�G� go.sum    g�~��wug�~��wu   ' �o  ��  �  �  �YtQ��S:��h%5��֊��^ golangci.yaml     g�~��wug�~��wu   ' �s  ��  �  �  �����̔�曑t�e�� 5internal/article/configurator/article_configurator.go     g�~��wug�~��wu   ' �v  ��  �  �  ����D�	�p�þ�Ɵa 9internal/article/delivery/grpc/article_grpc_controller.go g�~��wug�~��wu   ' �x  ��  �  �  �Gԣ8����Z��k�B��# 9internal/article/delivery/http/article_http_controller.go g�~��wug�~��wu   ' �y  ��  �  �  �(0�,7���SW�`�-M� 5internal/article/delivery/http/article_http_router.go     g�~����g�~����   ' �|  ��  �  �  �~}�?�a�v&� ��S�PEN 4internal/article/delivery/kafka/consumer/consumer.go      g�~����g�~����   ' �}  ��  �  �  �w����{���j�d4��UA 2internal/article/delivery/kafka/consumer/worker.go        g�~����g�~����   ' �  ��  �  �  4����_jE�˒L�'R����� 4internal/article/delivery/kafka/producer/producer.go      g�~����g�~����   ' �  ��  �  �  �����X�*�;\o4q� )internal/article/domain/article_domain.go g�~����g�~����   ' �  ��  �  �  ������W%�\����Ʒ *internal/article/dto/create_article_dto.go        g�~����g�~����   ' �  ��  �  �  �,�9�!��"p���=�� /internal/article/exception/article_exception.go   g�~����g�~����   ' �  ��  �  �  Ԗ��4���p� ~���"ѿA� internal/article/job/job.go       g�~����g�~����   ' �  ��  �  �  ��A���O�o��E�k���� internal/article/job/worker.go    g�~����g�~����   ' �  ��  �  �  -��@��8M܌'���k2�' +internal/article/repository/article_repo.go       g�~����g�~����   ' �  ��  �  �  ����d�F�'���Szt >internal/article/tests/fixtures/article_integration_fixture.go    g�~����g�~����   ' �  ��  �  �  6Ѳ�C��=�H�QD�|�Ngs :internal/article/tests/integrations/create_article_test.go        g�~����g�~����   ' �  ��  �  �  �>;f>f쒦�Ϟ]�L}"� +internal/article/usecase/article_usecase.go       g�~����g�~����   ' �  ��  �  �  �cs���h�iB
�!�Ɯ�%�� 9internal/authentication/configurator/auth_configurator.go g�~����g�~����   ' �  ��  �  �  ��x!�E�KI�29y����6A% =internal/authentication/delivery/grpc/auth_grpc_controller.go     g�~����g�~����   ' �  ��  �  �  T.�2��� �L]���b��Zm =internal/authentication/delivery/http/auth_http_controller.go     g�~����g�~����   ' �  ��  �  �  �v����l������W|�/ 9internal/authentication/delivery/http/auth_http_router.go g�~����g�~����   ' �  ��  �  �  �m)a�����b!��3K�޸� ;internal/authentication/delivery/kafka/consumer/consumer.go       g�~����g�~����   ' �  ��  �  �  �T,�&6�<�6
ka�
p�wT 9internal/authentication/delivery/kafka/consumer/worker.go g�~����g�~����   ' �  ��  �  �  4�Ya��/�)��U�R�;� ` ;internal/authentication/delivery/kafka/producer/producer.go       g�~����g�~����   ' �  ��  �  �  %�t�Ji���K�9h"��2 -internal/authentication/domain/auth_domain.go     g�~����g�~����   ' �  ��  �  �   � 'V�* ��;X�	n�ݠ" .internal/authentication/domain/model/client.go    g�~����g�~����   ' �  ��  �  �  W���^���M���A�_?�|� .internal/authentication/domain/model/common.go    g�~����g�~����   ' �  ��  �  �   �����K���ӓ�����; ,internal/authentication/domain/model/role.go      g�~����g�~����   ' �  ��  �  �  ���ں����z�^�%O��&� ,internal/authentication/domain/model/user.go      g�~����g�~����   ' �  ��  �  �  ��I�9Nr2�AO.z��� .internal/authentication/dto/change_password.go    g�~����g�~����   ' �  ��  �  �  ��Z���� AR4��RL#� .internal/authentication/dto/forgot_password.go    g�~����g�~����   ' �  ��  �  �  rΛƂ��@|e�.��ԯ��) (internal/authentication/dto/jwt_token.go  g�~����g�~����   ' �  ��  �  �  �[hE�h��B�ιڟ�:kM� +internal/authentication/dto/register_dto.go       g�~����g�~����   ' �  ��  �  �  Ɩs�U�<1��O�b��=:�E .internal/authentication/dto/update_username.go    g�~����g�~����   ' �  ��  �  �  C��c��ޅ
-��_:?� 3internal/authentication/exception/auth_exception.go       g�~����g�~����   ' �  ��  �  �  ԡG�J�+Zpy!���	�. "internal/authentication/job/job.go        g�~����g�~����   ' �  ��  �  �  ���a/ 1�^`�UG O@�� %internal/authentication/job/worker.go     g�~����g�~����   ' �  ��  �  �  KS�t��tꡓ�X���K��� /internal/authentication/repository/auth_repo.go   g�~����g�~����   ' �  ��  �  �  �ze��)�r,G��#d�ѐHS 1internal/authentication/repository/client_repo.go g�~����g�~����   ' �  ��  �  �  �'�*�St�0�x���׿Sg  *internal/authentication/repository/role.go        g�~����g�~����   ' �  ��  �  �  ��bM�8��|��F"Mc�� +internal/authentication/repository/scope.go       g�~����g�~����   ' �  ��  �  �  ���'uO-�>p���G��Kha *internal/authentication/repository/user.go        g�~����g�~����   ' �  ��  �  �  �4 k����2���.���"&B Binternal/authentication/tests/fixtures/auth_integration_fixture.go        g�~����g�~����   ' �  ��  �  �  6Ѳ�C��=�H�QD�|�Ngs >internal/authentication/tests/integrations/create_auth_test.go    g�~����g�~����   ' �  ��  �  �  R*H�sMU��P���%�&��L /internal/authentication/usecase/auth_usecase.go   g�~����g�~����   ' ��  ��  �  �  [T���BP|�j��!C$�/ 2internal/authentication/usecase/change_password.go        g�~����g�~����   ' ��  ��  �  �  ��'|�I�&�V���l_�+� 1internal/authentication/usecase/client_usecase.go g�~����g�~����   ' ��  ��  �  �  b,��}Xհ\��ƨ�Y�]�G0 2internal/authentication/usecase/forgot_password.go        g�~����g�~����   ' ��  ��  �  �  �5����-^�Y�\���ҡy��� +internal/authentication/usecase/register.go       g�~����g�~����   ' ��  ��  �  �  䒽=�7{�dQ�pE��#�P� (internal/authentication/usecase/roles.go  g�~����g�~����   ' ��  ��  �  �  �uhF�k�L�Ý�GdL� 0internal/authentication/usecase/scope_usecase.go  g�~����g�~����   ' ��  ��  �  �  	s~:ȦYŵW�<���w�kl�� 0internal/authentication/usecase/users_usecase.go  g�~����g�~����   ' ��  ��  �  �  �t���pĬ$m@�v��o� ?internal/health_check/configurator/health_check_configurator.go   g�~��>5g�~��>5   ' ��  ��  �  �  ��4�p�O[�0cwO�%1�sc�� Cinternal/health_check/delivery/grpc/health_check_grpc_controller.go       g�~��>5g�~��>5   ' ��  ��  �  �  |C��KbxvE^���ὼ��=� Cinternal/health_check/delivery/http/health_check_http_controller.go       g�~��>5g�~��>5   ' ��  ��  �  �  �>�k��aBe��M����=d� ?internal/health_check/delivery/http/health_check_http_router.go   g�~��>5g�~��>5   ' ��  ��  �  �  h$&�+���wlS£�9Q0��c� 3internal/health_check/domain/health_check_domain.go       g�~��>5g�~��>5   ' ��  ��  �  �   ���QU�6����9h"( �� -internal/health_check/dto/health_check_dto.go     g�~��>5g�~��>5   ' ��  ��  �  �  {��ߚ��[�)py����g- Hinternal/health_check/tests/fixtures/health_check_integration_fixture.go  g�~��>5g�~��>5   ' ��  ��  �  �  
&W�)�,IGg��D�a�c� ��� =internal/health_check/tests/integrations/health_check_test.go     g�~��>5g�~��>5   ' ��  ��  �  �  �������h^���B�>G7�� 5internal/health_check/usecase/health_check_usecase.go     g�~��>5g�~��>5   ' ��  ��  �  �  &s�Y8�^
CK��K��hQ Ninternal/health_check/usecase/kafka_health_check/kafka_health_check_usecase.go    g�~��>5g�~��>5   ' ��  ��  �  �  � NL���㹨���f�ݕ�� Tinternal/health_check/usecase/postgres_health_check/postgres_health_check_usecase.go      g�~��>5g�~��>5   ' ��  ��  �  �  d��#�y�@�H�c��	*�H� Rinternal/health_check/usecase/tmp_dir_health_check/tmp_dir_health_check_usecase.go        g�~��>5g�~��>5   ' ��  ��  �  �  P>��yFI����+S���@ 1internal/oauth/configurator/oauth_configurator.go g�~��>5g�~��>5   ' ��  ��  �  �  _����#�L#���І�A1�� 5internal/oauth/delivery/grpc/oauth_grpc_controller.go     g�~��>5g�~��>5   ' ��  ��  �  �  #����K�U=��ih;�kȎ�Cd 5internal/oauth/delivery/http/oauth_http_controller.go     g�~��>5g�~��>5   ' ��  ��  �  �  �ٱ��(w�,q��Ң�LģY� 1internal/oauth/delivery/http/oauth_http_router.go g�~��>5g�~��>5   ' ��  ��  �  �  ���C�S���A�`A�%|1F� 2internal/oauth/delivery/kafka/consumer/consumer.go        g�~��>5g�~��>5   ' ��  ��  �  �  �l_,h\�q�;Y/� � 0internal/oauth/delivery/kafka/consumer/worker.go  g�~��>5g�~��>5   ' ��  ��  �  �  ,�Bݹr�r�)�"a�K�v� 2internal/oauth/delivery/kafka/producer/producer.go        g�~��>5g�~��>5   ' ��  ��  �  �  DDD�;�5�5I�r���m��� +internal/oauth/domain/model/access_token.go       g�~�΀ug�~�΀u   ' ��  ��  �  �  �x�'!;��-i��I<Iv���3 1internal/oauth/domain/model/authorization_code.go g�~�΀ug�~�΀u   ' ��  ��  �  �   �@�>|��N��&k:�ʪ��� %internal/oauth/domain/model/client.go     g�~�΀ug�~�΀u   ' ��  ��  �  �  X���L�l��� �2gR�~ %internal/oauth/domain/model/common.go     g�~�΀ug�~�΀u   ' ��  ��  �  �  E�$�H
�s�{@�(\�$��8� ,internal/oauth/domain/model/refresh_token.go      g�~�΀ug�~�΀u   ' ��  ��  �  �   ��Q�9�+�3�LS�Z�"y)� #internal/oauth/domain/model/role.go       g�~�΀ug�~�΀u   ' ��  ��  �  �   ����t�4ʞy�.�]��1�� $internal/oauth/domain/model/scope.go      g�~�΀ug�~�΀u   ' ��  ��  �  �  ���k^k��?W��Pa� &internal/oauth/domain/model/session.go    g�~�΀ug�~�΀u   ' ��  ��  �  �  >��p�z�vX��RVv���Q�* $internal/oauth/domain/model/token.go      g�~�΀ug�~�΀u   ' ��  ��  �  �  /��rgxu�/��\9�,(-4 #internal/oauth/domain/model/user.go       g�~�΀ug�~�΀u   ' ��  ��  �  �  Â�?�^.�$K~:��Q�b� %internal/oauth/domain/oauth_domain.go     g�~�΀ug�~�΀u   ' ��  ��  �  �  	�0��ߠg����C�-u~� &internal/oauth/dto/access_token_dto.go    g�~�΀ug�~�΀u   ' ��  ��  �  �  �ҧ��e?�Ҩl�\����� � 2internal/oauth/dto/authorization_code_grant_dto.go        g�~�΀ug�~�΀u   ' �   ��  �  �  �n]�l[J���TT;�~Ŀ�UL� %internal/oauth/dto/change_password.go     g�~�΀ug�~�΀u   ' �  ��  �  �  ���y�m�������d� 2internal/oauth/dto/client_credentials_grant_dto.go        g�~�΀ug�~�΀u   ' �  ��  �  �  irF�60�u�!t�ˉ�� %internal/oauth/dto/forgot_password.go     g�~�΀ug�~�΀u   ' �  ��  �  �  �~&��'X�f���T$��$ $internal/oauth/dto/introspect_dto.go      g�~�΀ug�~�΀u   ' �  ��  �  �  �xE��A��~�Ra5B�T=A internal/oauth/dto/jwt_token.go   g�~�΀ug�~�΀u   ' �  ��  �  �   X�`�ԻxvRk �O"��
 internal/oauth/dto/oauth_dto.go   g�~�΀ug�~�΀u   ' �  ��  �  �  Η�Jׂg�9�1J9K�lg� (internal/oauth/dto/password_grant_dto.go  g�~�΀ug�~�΀u   ' �  ��  �  �  
_� kUNۭU����ހ�5� 'internal/oauth/dto/refresh_token_dto.go   g�~�΀ug�~�΀u   ' �  ��  �  �  �d����~=����ܷ=���C� "internal/oauth/dto/register_dto.go        g�~�΀ug�~�΀u   ' �	  ��  �  �  �艳���H�n8D�j@嶀 %internal/oauth/dto/update_username.go     g�~�΀ug�~�΀u   ' �  ��  �  �  	�h�`1��� J$!�_� +internal/oauth/exception/oauth_exception.go       g�~�΀ug�~�΀u   ' �  ��  �  �  �d�1
���W�ǳ('�\W�}� internal/oauth/job/job.go g�~�΀ug�~�΀u   ' �  ��  �  �  ��xb��\Ήn�0֤	�@ internal/oauth/job/worker.go      g�~��µg�~��µ   ' �  ��  �  �  	�3­�g��@��
����n )internal/oauth/repository/access_token.go g�~��µg�~��µ   ' �  ��  �  �  K�lؾ#D7+QW���pȲ�� )internal/oauth/repository/authenticate.go g�~��µg�~��µ   ' �  ��  �  �  ;�=��#���x�-&��� /internal/oauth/repository/authorization_code.go   g�~��µg�~��µ   ' �  ��  �  �  �<�ZL�a)�t$?�p�^ (internal/oauth/repository/client_repo.go  g�~��µg�~��µ   ' �  ��  �  �  �a�ҡ�	��ch�ܠ-�b��0 :internal/oauth/repository/grant_type_authorization_code.go        g�~��µg�~��µ   ' �  ��  �  �  ��>,c'Gu4��u��SK�� 'internal/oauth/repository/introspect.go   g�~��µg�~��µ   ' �  ��  �  �  CX4l�2�(�{�px:b>X 'internal/oauth/repository/oauth_repo.go   g�~��µg�~��µ   ' �  ��  �  �  �*�XBݴ��˯A<BR��� *internal/oauth/repository/refresh_token.go        g�~��µg�~��µ   ' �  ��  �  �  �oI����>�Z�N�7X� !internal/oauth/repository/role.go g�~��µg�~��µ   ' �  ��  �  �  �z/+8�@3D�|�
�����A; "internal/oauth/repository/scope.go        g�~��µg�~��µ   ' �  ��  �  �  ��,z7��t�πt��� �0� !internal/oauth/repository/user.go g�~��µg�~��µ   ' �  ��  �  �  ���+4��}��;�K��zA  :internal/oauth/tests/fixtures/oauth_integration_fixture.go        g�~��µg�~��µ   ' �  ��  �  �  6Ѳ�C��=�H�QD�|�Ngs 6internal/oauth/tests/integrations/create_oauth_test.go    g�~��µg�~��µ   ' �!  ��  �  �  \o����iJ�4�,O~�E )internal/oauth/usecase/change_password.go g�~��µg�~��µ   ' �"  ��  �  �  ��E�|�
.��锐̉@��Q (internal/oauth/usecase/client_usecase.go  g�~��µg�~��µ   ' �#  ��  �  �  cT:얼Ϗ�p��E4�P8Fu� )internal/oauth/usecase/forgot_password.go g�~��µg�~��µ   ' �$  ��  �  �  6cKl�:����
F�� ,internal/oauth/usecase/grant_access_token.go      g�~��µg�~��µ   ' �%  ��  �  �  �V�u�z� {�b CU�� 7internal/oauth/usecase/grant_type_authorization_code.go   g�~��µg�~��µ   ' �&  ��  �  �  �0�H��{���'��i��X��� 7internal/oauth/usecase/grant_type_client_credentials.go   g�~��µg�~��µ   ' �'  ��  �  �  �ϳ���0[�e1���%��Z -internal/oauth/usecase/grant_type_password.go     g�~��µg�~��µ   ' �(  ��  �  �  ������qf���JﾟI[�0�� 2internal/oauth/usecase/grant_type_refresh_token.go        g�~��µg�~��µ   ' �)  ��  �  �  �����V�l@{��TtU�@' $internal/oauth/usecase/introspect.go      g�~��µg�~��µ   ' �*  ��  �  �  n���jˈ\ţ��nJ��p�� internal/oauth/usecase/login.go   g�~���g�~���   ' �+  ��  �  �  P?i ��+��GK���^ܞ�l�? 'internal/oauth/usecase/oauth_usecase.go   g�~���g�~���   ' �,  ��  �  �  �>�By\w'sj͘5'��C� 'internal/oauth/usecase/refresh_token.go   g�~���g�~���   ' �-  ��  �  �  ԥU�W��C�O�_�V�g�� "internal/oauth/usecase/register.go        g�~���g�~���   ' �.  ��  �  �  I�{���:��b�rベ{�Y7 internal/oauth/usecase/roles.go   g�~���g�~���   ' �/  ��  �  �  �;ф��?��%:�I1L�  'internal/oauth/usecase/scope_usecase.go   g�~���g�~���   ' �0  ��  �  �  ;��:�JaÜӚ��P�iP�
 !internal/oauth/usecase/session.go g�~���g�~���   ' �1  ��  �  �  	kb׿��'wpd��3���ٝ� 'internal/oauth/usecase/users_usecase.go   g�~���g�~���   ' �2  ��  �  �   e�W��S{Es������r�w�� main.go   g�~���g�~���   ' �4  ��  �  �   %�;�n4�BЉ:�y�ϙ�) pkg/constant.go   g�~���g�~���   ' �6  ��  �  �  #N4j?` 9i �W���[?�� pkg/constant/constant-temp.go     g�~���g�~���   ' �7  ��  �  �  NԬE�A�I˲��Vk�Ҹ�%�� pkg/constant/constant.go  g�~���g�~���   ' �:  ��  �  �  �6�Ey`b��	L���gd�]3PP +pkg/constant/error/error_list/error_list.go       g�~���g�~���   ' �;  ��  �  �  7p�;��4X�Ɂ��Gբ��)y !pkg/constant/error/error_title.go g�~���g�~���   ' �<  ��  �  �  �?��ӛ$���Va�����- pkg/constant/httputil.go  g�~���g�~���   ' �>  ��  �  �  �wp�׭�BR����`
u?�>M pkg/constant/logger/logger.go     g�~���g�~���   ' �A  ��  �  �   {Hޡ�a����b��:��7v�  pkg/error/contracts/contracts.go  g�~���g�~���   ' �C  ��  �  �  ed};��h��;����	� +pkg/error/custom_error/application_error.go       g�~���g�~���   ' �D  ��  �  �  V,̷ >5�&F�q���Lf n +pkg/error/custom_error/bad_request_error.go       g�~���g�~���   ' �E  ��  �  �  8?6�¬�����d"���P`� (pkg/error/custom_error/conflict_error.go  g�~���g�~���   ' �F  ��  �  �  ��$c�!�������u�V"��% &pkg/error/custom_error/custom_error.go    g�~���g�~���   ' �G  ��  �  �  ֡��Ut�`��t���L2�T &pkg/error/custom_error/domain_error.go    g�~���g�~���   ' �H  ��  �  �  G�|��z�Ś��Th( �=B (pkg/error/custom_error/forbiden_error.go  g�~���g�~���   ' �I  ��  �  �  �:�1Nf�:o�� +�J�C�� /pkg/error/custom_error/internal_server_error.go   g�~���g�~���   ' �J  ��  �  �  V5�Jf��|b��� 	k�0i�! *pkg/error/custom_error/marshaling_error.go        g�~���g�~���   ' �K  ��  �  �  8�X7�4i�Qx�F�S�X��X )pkg/error/custom_error/not_found_error.go g�~���g�~���   ' �L  ��  �  �  tɸk��=zQ:�ҎĖ�ެW ,pkg/error/custom_error/unauthorized_error.go      g�~���g�~���   ' �M  ��  �  �  tݛ�DHZ.o���%�%�S# ,pkg/error/custom_error/unmarshaling_error.go      g�~���g�~���   ' �N  ��  �  �  ��s� lv_���9I\R�� *pkg/error/custom_error/validation_error.go        g�~���g�~���   ' �P  ��  �  �   ��E4��WhgK�?�%]�� $pkg/error/error_utils/error_utils.go      g�~��G4g�~��G4   ' �R  ��  �  �  ���X���
��%9�7�
� #pkg/error/grpc/custom_grpc_error.go       g�~��G4g�~��G4   ' �S  ��  �  �  ������~�5Ŵ�\{�� pkg/error/grpc/grpc_error.go      g�~��G4g�~��G4   ' �T  ��  �  �  
�#ѱ#c�gp�5������9 #pkg/error/grpc/grpc_error_parser.go       g�~��G4g�~��G4   ' �V  ��  �  �  ����H��׼�[dQą���� #pkg/error/http/custom_http_error.go       g�~��G4g�~��G4   ' �W  ��  �  �  |S1�
�ws6�3�LB�� �� pkg/error/http/http_error.go      g�~��G4g�~��G4   ' �X  ��  �  �  i�2*BZ�i�Ėd�G #pkg/error/http/http_error_parser.go       g�~��G4g�~��G4   ' �Y  ��  �  �  ��e |f>��#&b1�"�(��) pkg/password.go   g�~��G4g�~��G4   ' �[  ��  �  �  tҾ�����8Ί~3%vvi�y pkg/response/error.go     g�~��G4g�~��G4   ' �\  ��  �  �  9��@>�����]g9ܸ�`�*��� pkg/response/response.go  g�~��G4g�~��G4   ' �]  ��  �  �  *�F��9��p���P|���Ʈ 
pkg/sql.go        g�~��G4g�~��G4   ' �^  ��  �  �  �UiQuK���_F~�g+ʍD pkg/string.go     g�~��G4g�~��G4   ' �_  ��  �  �   ������C,�k:�;�Y�� pkg/utils.go      g�~��G4g�~��G4   ' �a  �   �  �    �xˡ{uO(1jx��¥�cs3] web/landing       TREE  c 219 11
i���vͱ�j�	�F�	��db 21 2
C"�҈^
׉�Kh��'6*�fixtures 5 0
�iq钍^(s�9\`�W�migrations 16 0
�[P��<k��� �B:k�׿mapp 1 0
�?��ֿx_|�c�XoX�cmd 3 0
�Y�`VYd���5��	��pkg 33 3
8��Q�eO��0�d��gerror 20 5
7��$j� �E>��4���grpc 3 0
XyF���l�PYY�|��http 3 0
��|`{W�$K�``tQ D��contracts 1 0
����l<��y����$�a���error_utils 1 0
�:���io�*����[��custom_error 12 0
��:b�Gx�#� F�>�constant 6 2
�(t�55ʀ-d ���TH�*�error 2 1
���S'����2u��ؕ�error_list 1 0
���>�ʊ�E��i �fJ�logger 1 0
d1��7T��-�JgDX�response 2 0
���Y#ݯ��V�0�~y�(�web 1 0
bw��֚0�gc�ˑtgBКdocs 15 2
�8`�?�魃�����6�56requirement 1 0
0�}X0Hslb��L��6�w�|api-specification 5 0
�sRP̣���`�Bkm�����envs 4 0
g���n�xOn�V������"config 1 0
d����e��Ҥq�nO�f��external 2 1
�� �:�>A$7�%[^#�Csample_ext_service 2 2
ԑl=z�~�*$eҫ��x��=domain 1 0
߂������H���9�yusecase 1 0
�D�7W���m�!��AړUoFinternal 126 4
����<�YrmD3LL0�"�oauth 63 9
��Ԝ�Q��r��fb^�����dto 12 0
����j�|0���@����{wϣjob 2 0
�7v����ð�0�vS�3Gn/qtests 2 2
���{;rr�3y�&�U��Nfixtures 1 0
��5ac�׏���w*�#���integrations 1 0
�qq�)���`���}�/�domain 11 1
?�e�9��؉>�dH5model 10 0
��,2�^�]�n�/x ��usecase 17 0
a����6�-���3G
��yLrdelivery 6 3
�D�������^��2��n��grpc 1 0
YbR����㳗�5$d�%<�9http 2 0
Y���y��������lRU�- Kkafka 3 2
��8C˺`���]�@�+�#�consumer 2 0
K[s��h�mզ�4׭|U%Ԭproducer 1 0
��h�kf�A�@�dW��exception 1 0
p��j2�L2M���u2I�;repository 11 0
�&J$s�@�-mcP����@configurator 1 0
F�� �_��۾�S�ڲ��article 16 9
�u��Q�&9��a�S�%��dto 1 0
��¯����y�]����o�V��job 2 0
��wX�7F��e�o�>q�tests 2 2
��:��N;�Jf9#M�n��fixtures 1 0
���i��uX��G�>;��Nintegrations 1 0
,���uE�Us0��6�ξy][domain 1 0
��!�<O!�}s�	���usecase 1 0
o�K+,�80�D�n��;�讔delivery 6 3
��2��i��&VK�*�o�^.grpc 1 0
��	!w��8ҳ;)�_��U�http 2 0
`iq��;��V[�	�5�k%��kafka 3 2
�)(��}{*�(�S��#n�consumer 2 0
}�DX�o�2�$9L�k�P�+�producer 1 0
K�ř���
L�{2���zexception 1 0
!�a_B���d}#����<�repository 1 0
�-�X��� �� ��[x�configurator 1 0
<���W�qL��1euqdhealth_check 12 6
��]K����M����j:Ww�Tldto 1 0
@���Q�YV��%�,� 	nv�tests 2 2
VHxA̝n"���c���^\qLfixtures 1 0
�>�؆�?��9[h4^�!��integrations 1 0
���ࢷ�#ɧ�	Y��S��domain 1 0
)��tFR��n��'�����Lusecase 4 3
WI6�=k��u��ڏŻ��kafka_health_check 1 0
*F�#�C����'�6��itmp_dir_health_check 1 0
~�r��J�`G�p*�ꉇpostgres_health_check 1 0
�g`1:q
S�+x̠�	�8�delivery 3 2
t�J};��-���)hqgrpc 1 0
G�	06�2=��/A�[T�http 2 0
�V�S�Jt�o��(=���configurator 1 0
��p��K��fN����	�authentication 35 9
F�+�L�d�#���:���Edto 5 0
E������W��}IJ`�M�job 2 0
�A��v�gM�/�PwD�	�tests 2 2
=?c?���ނ�-S��NJ�?fixtures 1 0
����!�6�f�� �k(`x'Cintegrations 1 0
���R�؇��}<j�LZ�+^domain 5 1
����z<{�ws� ʟL�model 4 0
U�9�h��kV���o���9usecase 8 0
���o�ZD�ڹP��Ԉ��Hdelivery 6 3
�F��i�z/�~r@�m)=ߢgrpc 1 0
3|
�� YW�TѦ~k� http 2 0
�v��Ь0��Q
��M��=�kafka 3 2
��p3WyL� tJ�ހԼ|�consumer 2 0
�Ɠ{����	\��8��producer 1 0
��3!N��gv �K�MtK}-�=exception 1 0
#zc����C،��a:���<repository 5 0
��v�e��\5��lnMconfigurator 1 0
��t���� ��cj����ymdeployments 3 0
P�ۀ�!�����)?��	O��~][lcv�s�4&�7;
�ŝ�

// .git/info/exclude
# git ls-files --others --exclude-from=.git/info/exclude
# Lines that start with '#' are comments.
# For a project mostly in C, the following would be a good set of
# exclude patterns (uncomment them if you want to use them):
# *.[oa]
# *~


// .git/logs/HEAD
0000000000000000000000000000000000000000 c06b82170e29a4eb519c9cff2392b3a8285e5306 code <code@192.168.100.6> 1737195170 +0700	clone: from github.com:diki-haryadi/go-oauth2-server.git


// .git/logs/refs/heads/main
0000000000000000000000000000000000000000 c06b82170e29a4eb519c9cff2392b3a8285e5306 code <code@192.168.100.6> 1737195170 +0700	clone: from github.com:diki-haryadi/go-oauth2-server.git


// .git/logs/refs/remotes/origin/HEAD
0000000000000000000000000000000000000000 c06b82170e29a4eb519c9cff2392b3a8285e5306 code <code@192.168.100.6> 1737195170 +0700	clone: from github.com:diki-haryadi/go-oauth2-server.git


// .git/objects/pack/pack-8f65ebed1d2cf1e556abe96c315a170d6eb96e9b.idx
�tOc                  	   
                                 "   (   *   .   0   3   4   7   8   ;   <   =   ?   @   B   I   I   J   J   L   M   O   O   Q   U   W   Z   [   ^   `   b   e   i   i   j   l   m   p   q   s   v   x   {   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �   �           
                        !  "  &  '  )  +  .  /  3  :  ;  <  >  ?  @  @  C  E  H  H  J  O  P  S  U  W  Y  [  a  b  e  f  g  i  j  m  p  p  q  r  s  u  u  w  z  }  ~  ~  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �  �            
                       #  $  &  '  ) 'V�* ��;X�	n�ݠ"�y����=��Eex��)[tPl_,h\�q�;Y/� �udM��69�}��%�C�s�E������W��}IJ`�M��눕���Ұ���rs.��@�0�"Xy����[�W����C���S9�ß/��P���&6���>�ʊ�E��i �fJ���fS��v�5���=�7�d����~=����ܷ=���C���k^k��?W��Pa����hJQ�^���9Ql��证v����l������W|�/��d'JG��"ك*����,�gVK2Jv��u��s�r�P�t��p����:t�J};��-���)hq�j��T�4[ΞTV�@N�ȅ�����9ݢ��}�4}�Y��2ЬB�D�0~aNl����xb��\Ήn�0֤	�@L�sS~�G
Kg��I����bEf���<J�	�-�L�X�V�S�Jt�o��(=�����J�q(��,r�`F�^�I$�5>Zq���#\Mʉn-g��-���*\��^��r�,��l;���A!xtޏ�mZr��ߢO�V�{� ��Z�zPS��n�0�XW�ܼ���Y �}i*5~�Qz{��Wn65�낞�~DV�Te~�������Z�~?���_l?F�B��"e!
�]��.�����4lV�u�z� {�b CU��\���H��y�Ị[�_���q՝{�N�]N �I�x����K���ӓ�����;�����~�5Ŵ�\{��'c;���Al����n[j�Q�9�+�3�LS�Z�"y)�F�+�L�d�#���:���E�,�\]�R�c�P�/�ſ���,z7��t�πt��� �0���#l+����7��qچY/��?��W���`�'_2����2n*��O߷]���h��sK�ř���
L�{2���z�qq�)���`���}�/��Q�_A������g�����[P��<k��� �B:k�׿m���if��K�����`5��X7�4i�Qx�F�S�X��X�0��ߠg����C�-u~�h.��T�����ﲠd��k) NL���㹨���f�ݕ��?�e�9��؉>�dH5t���pĬ$m@�v��o��Ya��/�)��U�R�;� `�D�i����U�?AuK��jbw��֚0�gc�ˑtgBК�rFHm�4�u��^}��$&�P���js�/�����z/+8�@3D�|�
�����A;�P,���b��l�|��T������������1�o��S�t��tꡓ�X���K���i���vͱ�j�	�F�	������b+ݡV�/��&� �s� lv_���9I\R���`�ԻxvRk �O"��
�Z���� AR4��RL#�!�a_B���d}#����<�#zc����C،��a:���<#ѱ#c�gp�5������9$&�+���wlS£�9Q0��c�%�4ZO�R�X����!]t�F%*�$��a�B- ��p�_f�?�'�*�St�0�x���׿Sg '��oW�ֆ��0l��h-�(0�,7���SW�`�-M�(B������m�	�?&����/W(�G�Ǝ	�ג4� k��-��(��~.�cf�3�T��f��)-LeWÀH�jq�7�<)��tFR��n��'�����L*F�#�C����'�6��i*H�sMU��P���%�&��L*�XBݴ��˯A<BR���+j��tj0	r6p`z�Vcm�,̷ >5�&F�q���Lf n,��}Xհ\��ƨ�Y�]�G0,���uE�Us0��6�ξy][-�8��eʸM>r�&�H�-�]���O��PR�0��Xx.���yY�s��Eє&��'�:#.�2��� �L]���b��Zm/��rgxu�/��\9�,(-4/��6[�YU��{r	�њ�x /�?���}�ݴ#M�~�ND�0=�n���W��qy~0�}X0Hslb��L��6�w�|0�H��{���'��i��X���0��¡(捫���=e�cs�2�oy�&j��o��U�� ��3|
�� YW�TѦ~k� 3�O!E���W`��8����h�4�T%!��uL��@6�B�5pSǏ X��	�{ �w+N�5�Jf��|b��� 	k�0i�!5����-^�Y�\���ҡy���6�Ey`b��	L���gd�]3PP7���:{6��e�t!��|Ŧ7��$j� �E>��4���8aWe[����φZ�.A��8��.5_r�12�	_�ȧ568��Q�eO��0�d��g9ȵ^	�b*����tw�8z9�G�kp�l��1����+�:dY��_��ܢH���+:�1Nf�:o�� +�J�C��:�̃���e�hT����L�%;J/��HJa���f~̿�F;1�h��ҟEug4�A5;��;��Lu��XǙ����q��E:p;ф��?��%:�I1L� ;���������W���Gc<�ZL�a)�t$?�p�^<?F��n)E�%�͟]�Z��<���W�qL��1euqd=?c?���ނ�-S��NJ�?=r5��5B���(&�����"={��c�M�A�7-ZE�6\>��Ix\h0!��IhO���>�By\w'sj͘5'��C�>��yFI����+S���@>��D�a�76NyC )�F�>�k��aBe��M����=d�>糫�I�����F��f� ?6�¬�����d"���P`�?i ��+��GK���^ܞ�l�??��U�dM�!�G����\6�p?��ӛ$���Va�����-@���Q�YV��%�,� 	nv�@�>|��N��&k:�ʪ���A��3?2�\��S*�)�=A��|��_ݶ�5c൜5�%_B�uC)�����&	�g�a���C"�҈^
׉�Kh��'6*�CMg��Hŧt�z;��M�E�uC��KbxvE^���ὼ��=�C��c��ޅ
-��_:?�DD�;�5�5I�r���m���D�z�V�C��s3 �qW�3�Eؐ��XW"4�=i� E�6�_k?P�(m/�Q3���F�����'űↆAܶ�-A|F�=j��ߔ7^[���.�F�� �_��۾�S�ڲ��G�w>,%�ܦ�D+9厹ІrG���EE�
`i*%���=�G�	06�2=��/A�[T�Gԣ8����Z��k�B��#Hޡ�a����b��:��7v�I�| rP:��������~��I�{���:��b�rベ{�Y7K[s��h�mզ�4׭|U%ԬK�lؾ#D7+QW���pȲ��K�*]���PITR�t]`�cLX�m�U������'mL]#x���I좊���8]`�M75鿁�����k���1���>N�3j¶K"$L��M�ﾖN4j?` 9i �W���[?��Nj ��5D��S�T
�=B�N�k�=������;��yOL�wq�.H���Β<�PS�B<u�"I�.�h�iP���K���qΌ��A3+P�ۀ�!�����)?��	O�Q><��h/��<�GKq+*A�Q�h���n%"�����!b��S1�
�ws6�3�LB�� ��T:얼Ϗ�p��E4�P8Fu�T<��$a��w����8s�rT�_v�@��&]���������?T���BP|�j��!C$�/UiQuK���_F~�g+ʍDU�9�h��kV���o���9VHxA̝n"���c���^\qLV����i���PI�趧�WI6�=k��u��ڏŻ��W�)�,IGg��D�a�c� ���W�p���^?HA`.���)����X4l�2�(�{�px:b>XXNY�I���m���8�TӁXfӣ��׀��g�o�� ��XyF���l�PYY�|��X�����:�5ڵL>a6�ˁYbR����㳗�5$d�%<�9YtQ��S:��h%5��֊��^Y��~���]����i�aS��Y���y��������lRU�- KZO�MT�WJ`zئq���FK�[Q&�ř�-�мg�����l"[hE�h��B�ιڟ�:kM�[����U��p[	.ޙ�j�%#]ӛCR��.3<�g������^+UE�ʾ����(?O�g��`iq��;��V[�	�5�k%��`v�]JE�=^�qH�毒`ժC_��d"~E�}�ʄp�a����6�-���3G
��yLra�-Պ���OI����\ia�ҡ�	��ch�ܠ-�b��0a��?�QB�`��� hC�*yb�'�a9M��̑-J3�Tb׿��'wpd��3���ٝ�cKl�:����
F��cmD�<��	��D���Kcs���h�iB
�!�Ɯ�%��c��B'�RL��f,0
�-�4c�/�=�&z�m����E~�d !$�8EBCF���Q@b�b
d};��h��;����	�d1��7T��-�JgDX�d�1
���W�ǳ('�\W�}�d����e��Ҥq�nO�f��e���Ƕ��^\��g{�#���fD�O�?a��Qw֒�l�@�g���n�xOn�V������"gA?��ԕ����؞f|�M�g��V��U����R����	h�kd$�U8��=�n����irF�60�u�!t�ˉ��i�2*BZ�i�Ėd�Gj�J	U�Ś=���c!���l�4&_N)m�`}o �Go�m)a�����b!��3K�޸�n]�l[J���TT;�~Ŀ�UL�oI����>�Z�N�7X�o�K+,�80�D�n��;�讔o����iJ�4�,O~�Ep��j2�L2M���u2I�;p�;��4X�Ɂ��Gբ��)yq�/�c��(BnZ ;�ۂr853�D�Ё8�i2+D&Q�nrΛƂ��@|e�.��ԯ��)s�Y8�^
CK��K��hQsl�4����(N�QXu*�ړuhF�k�L�Ý�GdL�u����������VO�wh�v/��se$6��ԝ��t�S�vf�V������8oń�M�v�� ��h��7�����t�.w*����,vzS�s/-q)�wp�׭�BR����`
u?�>Mw����{���j�d4��UAw�~���
�]�����{ؗ��xl��OF˵�z�l�x�'!;��-i��I<Iv���3y�f|$CΓ�v$�g�C͢D�z[ͦr?W���\Z9��t,��ze��)�r,G��#d�ѐHSzr��I|Zo7-9�wb-T{��ɟ�o�!�%m���.�k}�DX�o�2�$9L�k�P�+�}�ص��j^C�9,<.� Y~&��'X�f���T$��$~:ȦYŵW�<���w�kl��~}�?�a�v&� ��S�PEN~�r��J�`G�p*�ꉇ4 k����2���.���"&B�����Q�d�+Q�J�Ѡ���S�T�?K���nϲV=9ހ���>)R5��!rEo��G���K�U=��ih;�kȎ�Cd����̔�曑t�e���'%�Fms��j�§cb�/��:�JaÜӚ��P�iP�
�?��ֿx_|�c�XoX��D�������^��2��n�ф����6L.e${x^�f
�)��~V���*�$���L�G���T��;��R�Ͳ��j�I'uߥ3�l�ȍ�j�$��]��9�XFn�\V9È2��X+g�0�?��[�RV��#Dr�RG��&2�6���u��Q�&9��a�S�%���O;�ʛ�e�D]s��b�����jˈ\ţ��nJ��p�ň�]K����M����j:Ww�Tl����_jE�˒L�'R����ϊ��]�ʁ��VG/��~����*n�+(��:�p�ދ!+d��������?�a��ߚ��[�)py����g-�1�:Ն��M��VJ8Qh���ں����z�^�%O��&͌�A���O�o��E�k�����$��'¼����S���bߐ�q����H�����2�.t��0�t_���0���҇�����Qَ�Y�K/�v�G��}R6�� �Ꭽp�z�vX��RVv���Q�*���d�F�'���Szt�x!�E�KI�29y����6A%�A�}w��lϽ	�-A�-��⏀�a/ 1�^`�UG O@����P�,0�7��?��י2߮%��	!w��8ҳ;)�_��Uݏ�
i��"F�i����G���
��Z�ZǮ8x�]�g�����C ��$ʌ���S��{�������&�-a���#+��xE��A��~�Ra5B�T=A�h�`1��� J$!�_���p�xHQK�=Eա��U���s�U�<1��O�b��=:�E���4���p� ~���"ѿA�����<�YrmD3LL0�"痣Jׂg�9�1J9K�lgݗ�#�y�@�H�c��	*�H�Bݹr�r�)�"a�K�v����*�����X��J���oD��3��2,J�\�-�r�-�K�F��i�z/�~r@�m)=ߢ�Q��,ئ�x�LSĿ;�ី�(t�55ʀ-d ���TH�*���Wg�k0`����� ��!+�����qf���JﾟI[�0����ee�|��&"�s��\L~BW�� �:�>A$7�%[^#�C�,�9�!��"p���=���7v����ð�0�vS�3Gn/q��:��N;�Jf9#M�n����i��:��WGgt7��=�H��}ثHo��:.��扞���w�~4XU��.���.���ա ]^���j���}:���c�R�>�������?r-F��p3WyL� tJ�ހԼ|Ϡŵ˸�ʹ�����9!")�G�J�+Zpy!���	�.��%�A�m�u��~8����k�)Op�q�dqWc���X�&J$s�@�-mcP����@�vp��͠x�?j"��C�GԢT,�&6�<�6
ka�
p�wT����l<��y����$�a�������j�|0���@����{wϣ��d��.���.����xq���,2�^�]�n�/x �ꋤ@��x)�-�����w���ǰ��Ҫ���PaԪ���U�W��C�O�_�V�g����3!N��gv �K�MtK}-�=�p�2e4c��4�CuIS7�ܘ��E4��WhgK�?�%]�ࠨ1�F����	�})洎�,��4�p�O[�0cwO�%1�sc����Cy�lA������Zm�Rc�H���^���M���A�_?�|̪a\i;���@��0����ǳW���V]RĖ�JϮ�י�q|Ū��N����l`a GQ�Ȭ�h�kf�A�@�dW��Ɠ{����	\��8�߮=��#���x�-&����D�7W���m�!��AړUoF����X�*�;\o4q��)�@"�������Hp� X'1���UU��H�5kNl��l����8����0�VPԤ_�Ah,w�>;f>f쒦�Ϟ]�L}"��F��9��p���P|���Ʈ�G{������<�t�J#�$���S'����2u��ؕ���(SBc�TXi9��E���Ig��wX�7F��e�o�>q��Y�`VYd���5��	��v��Ь0��Q
��M��=��J49��|�#~����!��*��¯����y�]����o�V��XK�Ń���1o�"�lQּ���@��8M܌'���k2�'��^&!A��>��|�J����t��f�sE�f@��9�-���t!�.�q���"�2v�^��������t���ܼTZ����ʺ��Qo ��;&��b�>��G���i��uX��G�>;��N���"Zu��B�D0�V�ݫ�`R���>�� ���Z�s����Y#ݯ��V�0�~y�(��ӌ���n����[����A끥=�Cp|�5�΢�N�k�)��Q���#���(^S�p�Khr��=�Aj1�sRP̣���`�Bkm������ȸ�\��Z�D&;J`��n������C,�k:�;�Y����!�<O!�}s�	�����L�R�#�/T����2�W�5�������I���O2ns
e�M-�	:FWss���Q�'�J���PO�>�Aƻ�e |f>��#&b1�"�(��)Â�?�^.�$K~:��Q�b�ã�(K�=�\��Y�v��1���R�؇��}<j�LZ�+^��{�z��@6�o}7*�B?.��&�͕c����W}}y�"HK�E�|�
.��锐̉@��Q�h�3��S �E����*(�I�9Nr2�AO.z����;�T���>��߾6�}NS���W��S{Es������r�w���z�޵�?xޕ z4������'�3
��̔!�;�Q�#k����>�؆�?��9[h4^�!����^5����e��8*e���+��#nc��?���%�y�UOϟ�4�5�Y�k���S'=���*��t�Ji���K�9h"��2��Ԝ�Q��r��fb^��������D�	�p�þ�Ɵa��������Ie�ѹqk��xɸk��=zQ:�ҎĖ�ެW��@0E� �k;]�I�e9< ������h^���B�>G7���)(��}{*�(�S��#n��yk%`!�r22�1���ɯ����!�6�f�� �k(`x'C˝ �ʫ�� �����U�t����d}!�m/�+�������b���N��Yl��g%@t�t��I��&@0�E�'�3ۡ��p5��μ��8�P�S��0�}��`�w���@��<7��� kUNۭU����ހ�5�ϳ���0[�e1���%��Z��5Y N���`���>F��t���� ��cj����ym��@�B�����s̪�.76Sэ4�K2�m	�6�����ё?I�Ae`|Y���>�7њ�D\��JYh�nS�mU�Ѳ�C��=�H�QD�|�Ngs�غ{��cUV��edԠ���Y����ࢷ�#ɧ�	Y��S���"�v����J�RI�m6F�>6Y���/O���e�I�?Sҧ��e?�Ҩl�\����� �Ҿ�����8Ί~3%vvi�y���'�,Q��8��4�ю����s�ϰ�_�H�٘�������v�e��\5��lnM���t�4ʞy�.�]��1��ԑl=z�~�*$eҫ��x��=ԬE�A�I˲��Vk�Ҹ�%��԰��<W��j����d'd2�!�|�4qvI~kI���;�L�}b�=M��%7�*�b��9T�֡��Ut�`��t���L2�T�����F��@ P�;�:���io�*����[��׊*2ԉ���F�} ���(���-�g���΃yrC9wJ�Y��|`{W�$K�``tQ D����2��i��&VK�*�o�^.���f��8�uR�򗺵ҡٖR�b��n,	vdRL*���Hٱ��(w�,q��Ң�LģY��Ҟ�o��u��>������g�$�H
�s�{@�(\�$��8��|�z�̒b�?�O�����ڟ�yO䊞�0��D��
�iF�V� ���2Bĺ����ȋ��>,c'Gu4��u��SK��ܜ�|�N�8F�d ���8���p��K��fN����	�ݛ�DHZ.o���%�%�S#��:b�Gx�#� F�>�޺���=^<s���q����Gձ�a-E��a�~(b<������t��ޑ+F��hI�����@>�����]g9ܸ�`�*���߂������H���9�y�'|�I�&�V���l_�+��0b殲@��CbX
'��z���f��u��m���r~-,��A��v�gM�/�PwD�	���H?\�+��I�����7��f���$T�eJO�,7���Z �r7�K����$)��v�𼀜�BD#դ�@��4䒽=�7{�dQ�pE��#�P����s  �|s5$OY��ڬ������V�l@{��TtU�@'�۰�"oO�F�	W�j`�.����#�L#���І�A1���Ib���F��/�z|�j��� �⛲��CK�)�wZ���S��g`1:q
S�+x̠�	�8��$c�!�������u�V"��%���o�ZD�ڹP��Ԉ��H�ǃ���c�g���ŉ�p ����H��׼�[dQą�����>
p��\a��Oc7x!��%1��L��qh����O�������W%�\����Ʒ�-�X��� �� ��[x��?bo9";2=��c*��y�m�������d��e�"�F&�Qy�J�6����iq钍^(s�9\`�W��;�n4�BЉ:�y�ϙ�)��C�S���A�`A�%|1F��艳���H�n8D�j@嶀�,h��ޚ?Z$�¨xJO��QU�6����9h"( ����,�=C{/��;�19������8C˺`���]�@�+�#�����n�Ɏ)�T�G^:���3­�g��@��
����n��5ac�׏���w*�#�����X���
��%9�7�
���E���g<Z��<�rc0��%�7��M1�[<�U�m��'���+4��}��;�K��zA �8`�?�魃�����6�56��'uO-�>p���G��Kha��<	8���k�-Q#������{;rr�3y�&�U��N�c�eMq�b��|A3�y/�<H�p�[�� �x�y,�Ltj����Z
��8�F>�f�~�e�����z<{�ws� ʟL����0m���P�R:�W7�D몦���ἅ�O�@�.w�������L�l��� �2gR�~�3�ږ#Z&��8�Ck�'��O���{��Ì~��u!��ϊ�e�[�f�;�dEٶ������� d�G,�_�n���bM�8��|��F"Mc������7E��d9�lé�R����|��z�Ś��Th( �=B�w�[6�W���k����X�}��G�����2�~mR�������ߐ�"�������|� ��h�We0@�?��f��.xG�<7�d�m~3�h�������>[��'�r�'Һ%j:����Э���_S��M[�I��!�i�����r���<��%�N��k���EϦ�S ��9eLN�گ>��sY�nVo�@p�)jSW����L�U���7*v|��^�<#�	b�N�5���]���Bhz&f*�'�򷅡ئ�ê
��3�u�g�@=��Ό�Q&�����w�����"A��g/W��bʾS6��K}/R�$�Fu��@�dX�����"I�͹R�;����/�����Z8��	���V�9_�6�c�!,�c�B��	�[X�fM ��w'N�'��Ҵ�jEֲtȘL�Yq+��6�]����R�g��ύruH#/��D�O��.�~��ď:G�b��'�s��<sC�a��6v�Jȥ�?zt�3_H�@RM鉇��O_�`ܖ�#xE��;=�P �����E�&�h��2d^��zh������I�O�PI���UlC�uT�Ղ�P\���8YAa��ҧ�n��f�KX^46�%N�U�-.���(Zw�E6!W-���(��6�RF��N����R����TG)�M�K�2=�.%H�K���5��KϮ<��:��CͲ��@�>{������7��hc�f��|Cb&��C3�I_*@P^ۇ�92�.\`p�������XY᫞JQ�K��aLu��a�:��qV8��7m~%��-�%���r��bסb'����^��pW�'��k#�peK�/�oշޫ�*�+��+q~�7a]��K9�KiK�����4�U�f&0:"j�ޯ� �P6^��<� 3���i�s&�ں���H�UK�G��N	��mc�L#�ED�u&w	��uh�h0���˔�?��qX��i$�U_` �|a
�q=Y}w��k�h���l�{�Wyvxm$s�����]�_5����>VR�@�s���,�s�K�R��cԚUTҏ,��B���!hj�S��C����SU��Z�Y�������m�t�J����TY����n�N(d�mrp*�t�q�Q82P�(Jv�0.���-��n�=C)`@F��	Re��y����9��ű�g@�E�u ��5�A��?m�4����ͻA['���!'l��iF�g�v�ܠ6�i�耣R�cUO�E�>��������k��!{�~·�b�Ħe6u�<7��������q�Ä�V�+�@4�ΰiw=����n?d�	�A�ſi�qǵ�	.�I�͹�%|��`�"%^߶l��!���z)�tB����	�/��X�8{퐮�h<�k+ޮ���A��h�_�"�E�I�uH$s*�*��T6}	���Jw�.�\�ߛh�sO�?��yɩ ��C�ED�'��_�rm����[:	'�1�c��
�����27�9P�"�a�	`�>��3F���n�GJ���A5v���>M�m��x�.���4��޾_0�Nh�`�*Ax�\�Q���	��& w/�⢿p<��M�\1@}����wy�/n��`�����U���8�C�D�C���G��8Ԁ~�D����'���H�x�$'Ҭ���ʼ>O.r��H�@�b�9>O�7u�W֧������{�l�-v��:����%�z(�m2�	Wn�K����6�"<8e��9�N�o�7a4�ʋ<���<V��#��}��d�~��ZY���_ŋyFD@ZT�=�R}z��&�O��Td-ݬr���÷t����;�����T�J�Jfk�x��HL �W-e߲{����g�9��~�3Ժ௮�x��Q>Sm�KeR������F�������*���HU����d�e�0=DǄ�� ����O?>����u-�a�a�X޵Q�n�7�q=��Rw�	єԀ[��TK�#=F0Tb���%�/限�xʮj�Ж,a��k�e�N������kG:��C�c������o��C�3��2���,�q-$nv )��a�;~�������N<��C�&5(������4^C�q��'�"���Z��x<T�G�i$�W�XJ��F�9OQ9��)�h�΅�[
���4�,L�H�s�.h
}w��Ȍ��f������UC�4 QsΥ�:�������(Y�YaY0��6 @p��ɮ'T�0<=N�l�v�@u�To�Oj��b:1Z�7� � <� ]o  �  �� +! :n �� ,[ y6 b4 ��  � f�  (� � D  4  $� f� { 0 1B G F, U�  I  �x  x fg I�  ڇ d  ; ��  �� IN U �  � a�  �� ez ��  � GW ��  �  ��  Fu  B8 � �� h �� U� ]� A�   9 �� �@ |� ��  H7 Y� G  � � �� r� ;  �� I �D H� *� 
 %J c  ߦ �y a�  2# i� H� S� 4` ��  0� ǲ 6l  �h  D�  ��  9� � �  2n  I;  L  �� �� g� 3(  �K �� d� O% �l :� ��  :� �� � � �� 5  � �P ̝ `1 `� �	  
� �k u3 �" I�  ΃ 1 �  J�  I� �q Y�   �C [L �x �5 g, �B JE � a� �� ^b  9w  	� Gl  _�  O�  �   � �� Y� � �# DM  �F Ť g� �* \� �� �� gk _�  � 0Y �� Ev  *!   V�  i  K�  �z  ~� � 4�  )�  � 6� �� X K  �� Qr M� �u ��  9%  �� ��  M; [  �n G& [N �  7 �  =W p  �-  3  � �c F� �T  �l J? �� � >�  ��  �8 `�  (� �� ð z�  2> �� ��  �� 7= ��  � l� �� ,� ]� b m �j  �� �� zS �[ 9I c�  T. /	 =� 	� � ?  >l �< ��  ��  fF b� `�  �� r� "% 6d #-  �V �� r( ><  � V�  ��  �� S�  Ռ  щ J ��  $� Z�  �Z  7�  � �� ��  �� ��  �a a � @�  �2 0� ?� � K�  8 *  �l �� � �� �� c
  �� [ �" � F=  �� F D�  )o F 	 z�  J� m  �6  �6 t� W ]� H�  W�  �� ]� � g� � U�  ��  �u z�  �c F ��  D� �� �# � ��  �  5 RE {B  ʞ � �q e  ^� _� P� ,� �� �  J �x  F` D� �� � �	 � �� ]�  �w  �4  �D    <I Hz  �5 �b �� �j  �  ��  *F ( -   �  ��  �p *�  F�  ̦  }  ;�  � O  \� � ^� g�    /r  V �� ��  �Z  B�  w�  Lz �7 �� 	W � 1� 2y J� �:  .[   @ �B [� 1� KY dQ D\  K� $ Xq  �!  K! �d  Pd R  �
 +� 1i  ��  C  J�  J�  C; � x� �  \  �� +�  � �/ >�  ��  ?� M� c�  lr �� �V  �* Z a�  �� ��  R
 �  
 ˌ X �C ?� ) �  �~ >  < \( 0� al � _ �� �F  � A� �. � )Y �L �j ��  �) 8� �e -	 C  X8  {# dp �6 ;
  ] �� �  �� s  B� Uo �� 2� D� �v [x Po  ��  �: a o�   :| �� ]G yc �- J  �� \�  *k �� �� �;  1�  I
 �`  Q� 0P  {M �i m� �  D� K g � �   Jq  �� 8N & ?0 � ^F F͏e��,��V��l1Zn�n�o�ZU�|?�<*B턛 NO

// .git/objects/pack/pack-8f65ebed1d2cf1e556abe96c315a170d6eb96e9b.pack
PACK     )�x���1n�0Ew��{�@�iJ*�"C�^����Hm�2��Mz����o�Z�2&�Y˂J,M|�SE�4�,A���^��:7JX(D��!qb¢���P�:����p�����)o�vZ������}�����D�C/>z��fc����U��l����+<��+<_=2Leر�_I�P�x����J1F�}�셡3�����"�@���)��J�,|{{U���]򅜜��49���Yb�Z=�8�sOiQ�O�hيw�����X�b��*(?����h&�.�gT^x��Tx���X?�"��yذ~7�cB��A9cGo�5p#����{l���/V�vH�חGX��^�H�"�3���?ŀ-�,��?��4��A|E��[-)u�+M��(�����oA�x���MN�0��>��TQҴ�v��8��F3�Q�
q{:�a�����\�*�AB�Ew	��|a��y'���;O̡Z�6ʺ�[����؝[i��MK}Ɋ�(L@G�֌o���I�g����xgJw������w�w>��Z8�9����CT*W$|lGV|��1��cZ�~�V�g0������ O��gz9�5�k����s�R�7xlBE��5/4+|��ml�x���Kj1D�:E�F��	Yd�ktK-[�/r���g�E�+x���c1D1"R)y
S��Z�\#טBneR�4�X�f��[�pf�|h�i�)��|&��r�|���'�'��:_�8����b_.e_��Dk�	)O���\�.���Uc�W�Z��G@����a��ط�m�yF�.��-�Sȑx���M
�0��9E�BI^~���Bn�x���ԆZ+!.���#���fjI��ޣ�.:c�m�ZTy��D	V(C����g����`RFKtpҫ�m�D)@�&`���V����/X>H�h�f,?s���]��c;��:���N�B������o�Ŷ����m<ױ[�}�FĞx���KJCAE罊���q��ۨ�6E�^�N��t���k2C�\s1�Mm9��݄\�.�rJ��ƽ�V�'��M���=a+�EW�Q�چB�U�0\�
?�iL8�Y�����'����cC���^�$gc�Qgx�Ik�������Ug\πDp�
�oױ�p'�r��s�K�m���S��x���;�0{�b{��٬!

���:Ă��1�'��o���jJ�FS��)�`<��g{#��HV������`�X��'�0��%��c�\2�{����R�n\?,N��f��q�/��],�z7���7ࠝ�j�Kn-�뫸�#�H��*Њ�M}I[I&�x��OAN�0��{�Bmlǉ�
!$8�'�hc���9����E<��J�3��ٜ�@��j]�p��
�5X[���ʖ�����֒͘(d���QTV��H�	!*tZ����R����<�/����Z�|0]��~B?L�A�Rn�r�\s�6v�9��,�C�c��y�a�G����d���陱��/p��+
}8�=��y�������C.�	z�����vk�n���>���)��u��x����N�0��y
�SҬM3M����&�b�KJ�i���cBp�b��$˟k&��h��}���4�SNYl�U^:�{�Qj%f�+h����ݠ���j[cj�����T�I��kH^�������a���;<N��ڥ�(��6�m�A)�BO\+�w_�����p<�x��r�8q��݋?��.yZ���\R�+p/_�]&�tƩ� Ә��}MG�B�.p\Z��j �4���T���G�rN�7z?��m�,� �|rpf(3��,�:/2���r�{�x���Kn1D�>E/!!��_GQĂE���n�� �Yp{F9�*�ҫ�Ua*�1 i�(�E�Ǭ.��ֻ��x6w�z0��/�O�)�X6�&Qu%D�z:�@)~�����.~�?�6������8��ܮ{Y�p���`g��fk�6���7�������<�����\+�+�jg��JM��x���An� E��b�U"���(��Ȣ����F���Xmo_��RW_���Kf���{ے�:v�\g���i�!�=qc��9 �<���YV@����)�w���Y5�֢��9e��G�7��H��8Ϙ��6���O�+��U�t�Ix�����k(��ˋ��\ �`IH@X�-�	�t�W�3oB��a��X�X0D(3Ø�%}����q��"N�=>�N�T�����X� -.w��x���M��0@�}N�=%�Ӹ!,���%*�Qs�a�,�'�ZTS��G"�ڷ�Y��q��z
��N��E�
�Q��BJC��ǈ}��{nB���}-p�S���'�w.�qgΏCZ�3��]�;�5�s���o��>BZ�!�{ݞ����i�Q����L9�x��O�n�0����-[vP:��4Eń2dA��Uҭc�#���]N�ຖL�W�%����ah
���!��p0�!�a�5C=P�%���t��~BO=�3��M�N+��1&��I�������#���~YP�#����Momg;x�NkU�Er���U`�'@��1]C|���e�_�kFY���M��l3/�2f�+\�����^|����y:S�|����? �RdgJ�
�����x�q�U� .}�@x�}��ΛX��<�ݣ�H�(̃�hl0;l3c�����Nz�J-�ⓎT�Tg��0����?a�co�-�,XH�w�xޱt��~J�T�H�5�W�;�p� �<~��o��sAل��~ʆv�*��\�_���P�v;(P�$G�؇>�y�&�W��\���l?�j+�U���H�n:��=p4uGO���c /��$Q�dQ�%��w1ȁL�>�,�lEQ,6�����s�;�ޔ5�>��`�A�(�%���Z��yWw����`&�¸�D{�*l�x�ٲ!IP9G�>8x�� Eټ������8�1�f�5M �m��Id�J�P-��J�ߛ�Z�#&Nتpa�*�F:� <ւ�sOF���tV�<[r\�Dy�M�T�Uh$�K��#�)Z�3���3�Y14���/�[��:kJ|9��H��%�v�Q��B���1�S�w~���y�P����WI�``�F���/ldg���,6���}JdZ����qgڴ�d�rh�t+~T}�0f�/�] ���c��7с��`@k����Z[#�Q�_:�gyu��M�XUr~��0㣒�]��K�3����'�C�J���*g��g�\��e�6��"�V���O;�L8+�{�{��oQF.Q��a0ЌŽ�/�����D���̦�1���fL�N��8�WT�W��N��K�X�i\�����ϒeH��гIF_�
�p?6�K��CfLtEs�ʽ-i�YzҞ�L�|��lx��k]�HW^��Y��?a�}����UG��F`fW�Uڂ?��6=B��+x340031Q�K�,�L��/Je�57}�����Vg?�sɰ�������(U79?77�H�e��U&��0,\�����W���GM�K�d-~]~�������ʐpu��L��#)Ju�j%ߝj)�U⛘����������"��a3�'0���* Xd5TM������^n
��z�uãm7s�=>&�v���, PH,(`0^�/��ڲ�#<a�}�_K�m͘�M��yH0!,2�޿��M��97H���ʂ}Ð�s���T��.-)���ߚ��v� %��Y��RG���g�������i��J���W��3��ݰ]���G%?iڟ����*?���E��v�]/�6_yr��;�E�f�Լ�b�tQ��K��ݯ��k
;0�뺹{���%�Ey�9��2L��`�XϪb�U5:N��34����r�S�7�T��n[=��y�B�[mL��ť�����jT��M�]��+����E.o�*r�ғ3!1Y��k����vUӣG�u�YqOf�=2g��y�>0Y[z�}\��s*P�r3������O�\�Z�~�����>m+�9bJAv:�œ'�����$��Hy�E""[��� �T�����Y�ғ���X��ta ����mx�r �������?��ֿx_|�c�XoX����6T�_v�@��&]���������?100644 go.sum Q><��h/��<�GKq+*A��
8�t!�.�q���"�2v�^���V_�)2G��x�[ ������6��X+g�0�?��[�RV100644 go.sum �vp��͠x�?j"��C�Gԓ
8����<�YrmD3LL0�"�V_��%j�!x�mP�n�0��+<qC����&!�Љ���z�����}���	������3Xפ<@ű�V\A5zW����htn��k�)F�콜���RP�b+ŗp����p��b�ݑxV�B���"_���Q%��p���X�4}K��*5�Đ�����/s�S����`�7��m"��zGqh�,�ل�e����(�c}H-Mx�+~�U�`(��K��������Ea�=/�OO��Z����:s���I���h�`��G|���a�cE�f�NFH���rC�o����f��p�� ��
�Z.2�{�F�5����`�Dx�}T�n� ��(Y�'i;yl�v��D]�*"p�O�Ƃ�SW����(�c�����x����(5m0w$�ޤ��7T��	c��4Y��t�!�\2���ԥ��M�"T �*ȕ��Ӊ��!�YJT�x>O��ry��E��0��D�U̪��1Z4���0袊Y��R$p��p����"�=�B)P\� ߡ7n�`�_�u�mb�ȓ��e����hy�yv��7ÞKԪ�%�o�3�Mh��F�5�z��i�n�GC<��44�'ti3�V0�f�Р�9�q {�Hʗpj��&�x�����ҦV�g�
��	"�nH`�Qw��X�2��Zk4"(-�5�Б����5l��a��!�Ʌ�W��\�Ѳ�>O�c�>�Ӊ���%���#�Bx�]RKo�0��W=���=��J-�/�ʲ[��9V`)��G:i�`���"SH�k�,cK�N��Gxl�������LW�9�j;]Ώ��v��+&3F�%������7��&=��
';�]4nt��(�p2�H�>^�dq��o�A>�|{>�1�Hz{7� ������O�Hg����{k���ޟ#L6�ɵđ���ܑ������@�9}`Hz��|&p�����αN���B�@�zw�XT�טP�/~�`��!�C�s�w�Y?�B�}E�*��?'q���ӈ�v�tW6+��m�
���0�Ek��9J�1��ev����܎;��Vo� ����[�7� ;{_��z�?q&��� '?�z��|F�L@S��+��ZU?d*Rx���H�Uk8�x��P���[�.�4�V�i�RLu.�d��ש,_a�����,�FR]	ީ�h��j��/d.�6a+�K�\U
8�\i�\�\A�Vu��O����J��(D��Qk ~����9I1�F�������J�f�*O��E.nRj�sY$�򂿊U!�b4vs�LP��8��ZV%�XV�V�L0���ЍlD\Ɇ�RU�0Z'"��q���Ъ��Ep���F�B*x�\�)���3�ŋTE��x�W�w�6���-�6�7{��;^�$� ȶ{�=�����B���7#c���u_ɋ,�F3���g��mM��j��Znڞ������<f��ʘ)&W�x�G�~�4�v�.D�b�D�J*���+w�����_^ԑ��Rm�u+��|�bG�yj�^k�n�k�9M?�Im'�x2��;)����MӲ㥲�~d�N�V��]�ӛ�X�гC�G�<6�v���I��3O.�hD��L������$wq�n�2���DS�\X�l7ƞhr�G2Az=:�)?�w�Ch'0
��[Ï�`�	�l�R����I,U�TxPO��¶���f�9hO��f�j.Eڰ�l�������Ak42�ݛ�h�S�ñ��^]�o�4�����O'''pu�q�HI?��u,�ΟK��8J�mC���G~�*!g���!�B��2�wN����|�0�_n:c�K��i��Z���^���9�����K��c-��}�����7�޻�D����>t�c@���ک#s�X�V��1��_����4�X�
��5D*�Ш��Db��>��� ����rA.��e4����}s8��L)5�L��(�c�5���[-f��j!���5���PI�`s_��a�W�#�)h�����"�݋ˠuyKy�� ���@f�9�E�}������i/v��5C4b㓭��N�KY�'��*B:Y{f��%ȇ���s�L�P�q�c���b����W�pJ�n@1��2
;=<�	g> |�������̒|�0*�~kxy����{h������ڏ�~~~׿��:Mt{�Z�2ʁ� ��=��Ҹ�m�j�&�v�c��لe
.E������,�D�H�Jтe�e��9�qW�g��������WL
B\�10�����M ���]�=1�	�-{W�:�`�� �#�JQ$����"�(���+�������$��L��[�f˲J�?�u�a�ۣU�\g)�35O���8� �������6��h�����.�/P�1�8�*����.U�%$����i������.6�R�?'Й!:Vh}~
	�T��k�w��&�����U�R�p1�F��5�]�{ם��?�M�,����b	xlk���̱(��C!�zQG�[Ʊ�1$��?��T���ls�*8�е?܏w��kO���P�=9��M���w��*4�PUr)��b�X"��%��]w��W���9 �Ƣe+����T���3���f�@�<8�Q���my���~<<���L�[Z�����ě��y�C�>��Dq}F�}:4Y~���� *�*�1�����20����x��~�$����h*�u�YG�� O^�?���L�?���3+=�G4_��>?�c�-n������F���;�	dT��љ��X�����v����+>�x���{�5�=�<zO�I��Ǻ4�W���@��$W
z���'��A$֥�F�z7�ģ���������N�$�ػ��ꖥ��g��d��%�?�L\S��(㖖�Q��l�/�����|o�|��J0T\vu�f ��A� d��aIoB��}8�a�3�;� ���;:1���G�x�wj�ND�+�Ϻ���y��Do�m}p�n��_j[��6����>��k�XS��\�Tۆ�AԞ��i[,�Qq9�|�\Y�ƚ%�R���N�Ӄ�k�'Ն��_uO�Q��38���?[���Ọ�zQ��&<%��T3��[���7���4���e��-��A�vNͫ��.L�L*}.t�#�r�*yJ��73A[t!>unB�I��O��6VC=��=x���tKiC"��^���_��Bqjj
��,�/!���PT�������d��'�ħ$�$n���� C�Ұ�x��X�o�6�_A8����v[07����N_��e$�b+K)�����;~H��}XbQ�}��x�v;������]q�y�+Y�A0eq�)�HE��V���J�M��6�rSf��2]�e�*��٦�*	�l�x^�j_
�x��LT����ř�3ڪ�/"g��51��	�M�VB���i�.���HB6��xQ$�����.�H�W�L��;���P�ez:��U��r��o�h_ �L�@��-HkÌT��n��x%�RW��&�W�gl���7&��+v?��γp��'^2FJ;�(E��� �q,�g�*2T��WҪO���!�����O�Cd��ӛ�V~VdG�n�E�VuN��LV{ؾ�p�1��\5��T����FA��S#�@h- p��[�xm����3�+l��ե��q�V�ma�b���N���ξ� �f��ȡ�s�
�l�jpH�Lnde�O�DV,+�k���y������[�� ��
G�3h���J���<�b�B��W�j#���i�r����S�=�������r�*O�45����>}J���n��RFۗQ��FSt���z�� }7Y��r,B,$��2�^�����.\j�*y�$A�A`�8�U�&���@�!
:��c睰,�i����O�+K@��c{�d4y5�k�"�v�r�H?�Տc���J�+v�7@鞞�y�螞���4���C�l.)�Ag�����a�P?���#D��U�~��e
�!�T��-����`�X�gT�!?�;{�9��M�;>kL�ǀ���:���͞M�3�3��Ơ�c�P��D �����1\�Q̳����%m�m������C<-!	�R�'l�T8��ֽ'*U�����+�%����ꘗ��2�}���#���L��/�:\.��_-�����<<2��o�Kmt��"��I!~mE�9ޥ;��x���u�����꽏�(l��0�������<��;�>h�����+"jAB�.Ψ)�n��Z�}(�O!a��7H��Y0��S�ldΊ<ۿ��*,B`ض���x ��=Ĩ{��.(�N<Wk(O���`R/m�D�c%K�=pd�yߎ*t2XxMq��ǘ1 �~�u��f�s�lD�fk��cC{�ԇE;������g�-���7�ty�ha��1�֦w����MpIo�p���P1��sy��#nܵ�?��"�Ѝ��8�7U��4�=}��lz�A��Ѱ��|�3�8q�������++�	Dc�!��n|��Up7���!�"��E��TĠ�h����"�����<�?v2�<�`�X�(��Ԇ���� )�Mిj�b�;�n����`���*u�r�C����~'��?��L��'�.��|�eWJ��s�h�H�HP�4:3O8�R��q-�v�|NF��
䡡,2P���,+vh����$�Mu:�iN��jm-
������v�<Y�%��£�#L�^d�Z⚪a���n����K��.4���|	P���"2�9{��GL��[��!h��a�|`�uK���l�D���^c$��st0\AIm�������mE�u���h5�D�[�_3�5e{�.�LjO�~r�M��	����������C"��,����K���ۥP�3H:��.�%�UD޸�G��E�Bkc�a?����KI�*@F��Ɂ_�W��=����Ӂ�@�o�]�8-���A���%֬����x�340031QH,(�K�g��o#�3���*�z�fY��T �K��x��VMo�6=K����2�T{u�C�ަ[$A7ȡ(��$b)R�(����3��C��u/���y��p�T�ņג�KS�v�yB�$�xy�3��7�5�-J�Q�w[^�⋷V�XV�~��4����k��F�g��[��6\��)*�%p�ƫV����k0K�Z.���qo�&}mO[%�=���4��P��ń3E����\�f�H�y6Eĺv��!�r�����4O�W�� ��\����Y���+����<M�����#�w��wiZF�w�3��	�ܦ��~p�|S���Pr�a0`,�����7"�R��s2U��a��^A�����y�&�� ��IA\r�8ZKw�܂��nq��{�{pUy���@i�_��f� ��-���ʔ��e�Q$EA.d� ����}��5��z�mXe���e��a4��'d �qW��7���@4���$c,�_@�D���^���9��m�.gGU�2�
2گ�Hw�c9��=@0���M�M��?����5r{A��o;��~�j�E�7aDD(��X�P���Y`���CA��zI������sb��<%��X<{��Xl,@�gd��C��l�ߑ�8��VNz^���(D.�DE������HF�H/�(]���niR[����>�PL�`�]'.��=�"?��?�΄��b�C��Y`�hF�9Jh�3r�r��B>s�?Vxӓ$��N0Ww!3��^����}w��|݈����kPu�|�pCl�.�� ?�i|��;�U��h�@�5�n�`6�b�r���Շ���K-��'���St>KnFvF^�ҘO(�52A���h�8��nBB����gm*K��S��Q.���v��-ޯS�%v�p<��Wb��?���]���چ��k�y��ISu=��p�������zD�s�/����4�m���h.@���8x�{!�Ft�k~biI�d7FG!0K?9?/-3��(�$�h��N�Ԣ"[�2Iθɷ9y�C�@x�[#�Al�a&^ϼ�M-ǂ���R�9�RKJ��'odʮO��+I��K-R��U�t���<���Z.��d�̼��D���<��"��dTթ%�%�z\���� ��@�������b0ǽ� �pM��2��R�=�5&/d�����
 ��<���x�� �[|�KbiI�䅌�� Fj^IfrbIf~�~r~^ZfziQbI~�Fyn�Ԣ"[������l�x340031Q��OL�OI,I�K�g��Ln�¼�'�4��v��g�8kQW��_Rr$C��ܗۃ.�.��0�]K��8��,�F�7����,�"����#�aɹ] �)N�nx��ϟ-�����M�6Q�Km��v?P �U��%x��ϟ-��_�[����䴭��C�m�N�p ��s��x��VKo�8>[���U�T��)@��`�u[�ƞv-R2�TIڛ�������i�f}��������lyy�kI�F$�jZ���,-�	�&�0����U���M�I�;�ğƶEm_V�&���PB]�����Bw�Z�P[��@i�@i[��=�Z��]���j}W����rW�����7���q���iy������&�F��fIr���-K���<����67�K2������R� �� �� /H� A Q&Xv��[dV��S�? �u����Z��!���Ԥ�;������
�LX;�io��p�n��\LmChV= _12�ޔdu��ȋI\�I�+CN|�E�f)��=��pW{���>88F���rRr��[*���krvN�̱Eܣ��d�*�619�ov	�4M/�����H�cl�J�P.��Tɬ(�RV��]��W[�O��3���If0Z�-�a�-U��Ye�8���~�����A�/Pk��1�s�r���r����_���s��K�����Lct^޴��X7Z��Gf�A���#
y�u�D%�v�дHoߟ�׋�VqD����d���b{LYȘS�s���C��fd���}Ods虵�{U���#���X��C4�^v�l=�)5P�,b�Q��a�r��0�]�?�{������ ����rYIGښm>��-4t��c���#�z��jW��/g���b[�`ᬆ��6:V�/(�m�F��+^�p�]{-��i-�������z�ڰ �N��<^�Hˏ-R�S-�ش��S\�Ȱ��t*�h����>���,4 (5� �D�۰��C2�v�w+m��C�<�Ƣ�JGqD�k�mZ(�P�t�,&M�e���d�'r��(�:��΍��'
q�u~<,����1l�	1�sH��XB��:�hLQ��V�%"/7Zʖ���� q�^i����!:�M\N/���wR�I���!Y��a+^����*���������˷Vx��T�k�0��������-Ї-m(�la�{2��-�Zm+HJY��O�?�8�Ә������t�C{���R@V��z���"\��M��VJ�|�ř�����7�a�ϼ���#��Ns�(B�\�Vʮ�n�*8╪k���(z4b	���m�]�%<��0y[��$�PD:�N �*��4	9Ҕ��)�1Em��N��N������Y�p����!�סX�u��ߑz=y;�9�TJ�F��\���3�ė;�^���\I_����3��p�EC�����4�xW�N��q��}K�(L�Xؒ6�l@��J&
�7��/�~�D>�i���&��_";X�Z�F��x+�Z�[+4��_�߾���������S��cN�B�
g#�Mc'5�m����늗�Y�V˦����4�Z �TS�{�/�[<~ �
Y�%c2�iT�]�I����RՆ`�ʲ�ɺ���N��ja��
��������[n���3�-�<�?W'����pM�c�Q$��7�N��OC�v���!�Nk�������⠇�)�x�%�N9����ޅ��)x�R�j�0<[_����%�)�r��B)����6�,�X�Y�)mȿw�4P�P��3��Ѫ�b�JBaJ!�iHDW&�<)�n�����i��Ε�]��i��̱m�|��7N)IgD�V�wy�6��H��!����\��p=�����܋(z�r��x���#8^�b��#h�M�}ؕ�Ap��j����Y5��e��$�}�6�H�tv���7�g�����u���C���l�	'NR�%�L2td���#��?W%��3�����H[����;9�D$����/`6nv�,?�4�dI*"]A]���'F���_�0Ĝ�����l�)$�?`��gc��\��� �Ι��bx�����uB�Ds!�������>�Ÿ8!=���g0SC�k�'�U ����4x�{���uC�g^f��v�x 8�-�x�340031QH��K�L�K�g���d[�����E=k,氅\n �:��x��W[s�6~�_�2�&�da�Ӧ38��`;��t:^a�����LBw��{�|A6����}߹�ӑ����F�"Y����Td�\v;���M��b;��f�w2e/�赦�g�nC�=��_%D,@*MTk�g�@
Œ�b��Wݮڧ�u$D�,���XiJ���q�TƉRM$Tƅ�*ʘ���@5���4f��rY��kP?��35��@e�����(��u�;��;*Y�믥�(@�^�v�+�`�'��Y��G���2�(�g-�=�����s�eg�	���s��Fh�Cs�!�o�08,��"�8.%�gTq���4״�:$U����PA)��*��W�r�v,��JV �2&��
L>�%|#�&��{�oO<�d��Lq,%�U��W8�����:֓7���v]�s�?���޴(�!�MӃ� +�����;9��҉�� i� v?�	o&�Cgd��E��S�]�ج�ZdDmya+�+*��W�wY�1�R�ԃ� ]��ԙ��=\�f��ZWMd
�'q˛ 
Ry��V�nߓ-��$� 0��.U@��#����,���+P�b�Pj�3�x�&�K�w4����񆼿0	��J��0����i���X��DB	�\�-}����L���˷�v��`ۋ�(ZE��Z/vL'���1��	WD�8�ڶ��l,�M��R.IJ3Ń<�W�ɍ���YAZz˰�����&ˊJX��f��r��Ur�3�%A̱2�8��������a���x�B���*���1���A���gmku6�C�0�'@Sm�6|,���qy�	{��o�p����>����F>����R���G>-�E��>@N ���!��BA�P����B�1D
���E����z�(nI4cV�q0�e��f�	Jl��;M{L����vx��j�mnWƏ�OϔL��߂���3�:������a�N���d��Ű��7ԩZ�� ��u�;��}�=�5���w�Fe�����y���F$��6����C���a@3��C�<IwQ�i�	��3������<�}h6�&*���p6��f��<�34��$/��E��.�KC&���4�_R�
�yƻ%�%�:UO�<h빎Q����,�CcI�k���#����LZz�-?�#w^���$ ruy���q��%	{)~_^�f����b2��8�� ��S��R1LDҝ>�pB�'W�_�e���~�	<�.{�Hz�����7�	��1��m}�\��y�����
�` �N#�s��/k�Q�M/��H�YҚ^�D_��*����z��W�.����-�^��!�lP�������T?ć��?�����bx�k�[)7�U%=�$�4I/9?W?;5�8�2#3=�$�<�H?5�,9?/-3}cc ���;x�=�=N1�E�-� \�J�75R*��(2�ٍǎ�&B�w��G����
Ҍ�̛y����P[�WM�<��P��������^��z�4s�a�!��r
�j��I-����')�������J]E���.�a�#��R����. ��6��<&I�#��ؔ�(���c�v�2qfy��on�աc���_kP�(������j]8��0�.f��������9����W{�s���d���vx�ۦ�]`��Ērfǂ��7�nv�ea t�z�$x��.�Cp���WxHpjrQj��e�+$d��Y)e����%�R�ʒ���2ӭ��Z�]��\C�6�2E� �
w�x�31 ��̊�Ң�b�w��/'�Ɖk�[�$<�%&aV���^�X���W� �#:���M���}
ǝ���_ߟ �ңx340031Q(��I-֫��a�u�d`>��C�j+�&]�{�p���!DQqr~T��wBC��z�5��˙�1�w�l�����������������<����g;m=�gls$}/��L�|C֑����WQ[��[���Y�o����{mC�%=d��ũE��G���6[7/uV	�⮳5G��  J�Z6��6x� �������غ{��cUV��edԠ���Y�����HxݐAO�@����Фz� ]���M{�*�Rʑ,0J�]b�_�n�5jL4Ѹ����˛}��wG�ird����� hR��z ��X� e&��m�1-'щ��^�#���ȶ��t ��lG7M����(�q�Y:�%V�Y]a���a�=iQn�7�A��A����-ݨ<��>JT0�r�R3�oѾi��-ή�
^�O���so���u�9��ˬ�5��E��D��-ܠ3KR�YL�������r{~tv�)�&���7�ٛ�'�'��sL�m�m���?箦�yz�(J��ĭ?��LD���>x�k�\�,���\�
A�9�$Y�@��)�����bos�s)(�e��[Y

y���V
����E�ũE��X�&�2s�5�4N.b�*�� &� �Nx��1O�0�����2Kiqd��]RD1F��""Lَ����E� H�b��z�����bv<q�V�p�5b`E�����rLO��tr��t�Ӕ��c|����E�8��,㮒.���<)��Y��5h>ե�)4!�k.�͐x�H=Lv�N/�&�R�1�A�H��Bè�E�ŬlV��E�,a+��l�v]o��][o..C�y�/��nU�w�I�\}�=��ү���P�h���!��z��i���;�����+�xnD�Lx�͑�n�0�w��$O��	�Uj�ݚs+#ۨy���j#�j<�p��;�_J��4I�I)t��	'��#�P��"c��1L��T�
��0�a���W-�Yǩ<d�(��a�H��"��p<�>Co,���2S���Ʊs]�'�ܻ��KޤW��>�[�9_9.2��n�$�_�+g#Ɨ�G���h��&1ᔙ�o�(;X����6�ˬ-�F���YUQ�QV�55+�CE�E�A۝zh��a�
��f�~ا�M�_лP��+x�ՐAo�0����/��� VP�-�;�c�A�1�(�V��~���d�]�C�|��k^ú�g@�Ը���˼g�,V`�0{ �&P7@�����(!��EF�b�����yT%dd�ۮAȲxU�A���	ڭ�U�ھ���K2�y����TY�-��Y{w���%�0�_�y��8}Ȗ4N��{/�dY��b$�t�m�Ző�K�x��k);���F�]�l��UK�,/>V�gy<{��4�͟��5q��7"[Ln��B�M�㟻T��;��2�]�;�����nx�k���9�0��dG&�]�ܜ�]��}�<��sJ�<|-S+�����
"�C�<�},RCsJ3��+��JR�K�����R&[��N�ev�|�q�)YFN�bq���|� '�J��=x340031Q0202244�&���E�%��y���z)��yzŅ9��>����5g�n�u�Q7=�	�hHXiX��;>'��ErH(��a����d#�nK�̌��K�S��֞�O�U|��P���?:�oN��O#Ծ�/�d��N�8-�fw+�c][j#�6 �������">9'35��F�C�wXL����wj��]����k�ڹ���z�Kv��|Z��lK�*���>�/N�/HE�R�C�x��7�{'�0�Gp�ꭏ�����|�]	_���6���gdr�V�¢��(�(?�B��lwt�� ���ww�׳Egt�i�ڧ����/ޛ�S�nQ��P'����&m&�E�iE���%�٩yH>��?��"&�@��U�����D�e��&��$���<p.�6�}g� �����ɩ��8��`0ѕ���V��_>(0�������VϘ������Kvs�(�m�f1ڍL�KK2�2�K2����S�����*<��ٺ^]x�Z����u�q��������/|8�n�yK�V; �B[���(x�A �������O���{��Ì~��u!�$��H��N��Yl��g%@t�t��I���*�pb���0x�    �
x�]��
�0E�|�#S��u��!��[	�C��$?_[7�{��^QX�Y�F�d��_ �5��5�����X���8+�J���b <x��Ġ�>�;EJ���:AU61�'q(�.{HJ��S�RXJ������ɤ0βx�s	�Pqt�qU(-N-*�� 1�U�(x���KO�@����V���1��B��a�0�uC��Ԓ  S��{y�����$w��/��;nHN@�8a�,���x��S9.�.͙a���;7A�U�a �T�h��%s'�xW$�*W�ةd?�l,C�;�
wd5�Z@.ޔ�'tNh]N'6bF�c��b�뵥�����I{ua��TEvƚ���1�"שT�}���Rؤ�J!I*���8�1���3��:O�kՑ֕j��D�Lp꓈;���/��#Ǽ�8	��Q8DT�3��ԟ�K�17`���)9�+i]�d��86`w��.-r�]�A��`-rl�^�ȳ��P]��=3�w�_��Tx�kc��2��dFIN.����'�c| vCմx�s	�Pqt�qUH��L�+)�� =��.x����n�0D����N6`i� E{R!b˩L�I/+n"*�R��}�J";襇�����pv�L�J@�*�n�:��D�V\��A�u�,�m�ϓI�"V�@YYr��t��Ya�ǅ���.Ŗy�؝Mg���*��p-��c�nl�H}�oq�\�����l6�H��y*��b,���_dj�]�oyj��?�7#���S80?�~<0�8:���źeLEnp_{0��6�#ғ���Pp�����_`F&�k�!m�u[h�T>��z�ں {Gd�`z�"�)4Wr%6*^��TW�?֩�M�,�*F�!Ѯ1��c�����?t�xW8�ul�t�	��-h�=�v8�rD_wh*V�dw�{��a�����B����y�Żv^��m���yOU�	j�
x�{�z�uB���& r�x�s	�Pqt�qU(N�/H-�� 7���Cx����N�0��y�QN�D%��e9eS#�mS�:,�%2��Z�v6�Q�i
H�"������Ҝ$� �c$[�E�
��tɖ;'�D�ėQ���,�5#��
�)�~Be`��b)b�R`J��b�¤\�[,��Gc���<���7�?鳪Z��e�\td�lv0�@b��H��CH<��Ώ�6���$�����@��Z�ԨVv�����G8���X�.n�[�߁���	�md�����ɒ%��C�u���EF�2`I�ƭ�
�ܠ�|��@S�I��Z)��v�+b�������^�˶(�&,-�d�<�,��}��������C�nф�7��a�U����ٔ܁��)�������F�<�����7�{�޶�3���7��2����� ��Z�'ט��TF2��%x���q�e�kqr~A�D7{0CI!�1���1H���@S��?D�/��G!��30�U�K�RR���2J2�'�0Jj@3��SR�KsJp��39��d�?H[^b���sQjbI�Bb�Bf^Jj�B~��:��<u��4���R�����|���Ģ���J��������Լ��J.� W�WO?��̔�x�G@������ �@r}���5 |�U�j�Sx�;�r�e���[��x�s	�Pqt�qU(��I-�� 1xH��x�;�r�e�GQ~Nj���DG����T%�0� g� #M?��P�P?��PW]]�м���T�^�.���Ɏ�6�`!����%��
�i
@�Ē��<��V��r��9��%
�)�%�\��M�0���椂����Q�d�䧌V�`�ǃ�
�~`�k@<�i� �NF�x�s	�Pqt�qU(JM+J-Έ/��Nͳ� n����x�����}�{QjZQjq�D������;�3*C��K�S��s2S�J�3S���r����ԕ����Ƭ*��
L!���l wS5�[x��̾�}#K�kD�f	��L /ܺx�s	�Pqt�qUHLNN-.�/��N�+�� m~���x�����}�Q���Ԣ��%��PO�ɒ�z�;W0 ��	��*x�[���};#K�kD�faƩL -X��x�s	�Pqt�qUH,-��/ʬJ,��ϋO�OI-�� �n!��x�����1�Z��$#�(�*�$3?/>9?%�XA�k������G�&�3ڱ*��M<mͪ����h��T���Y��\_Z��������ad`���������5�c�d)&��IL.@S
R'dҜ<�Il��#L��s2S�J�3S��1L.cV�â�@���5���J��\ �sXQ��5x�k�hf�p���8�hb��deF����L e���	x�340031QHN,.N�K)Jԫ��a8�o�W̕���Ww6��[xqq�C�����"���܂��T�T�Tݜ�����D�ƈ/;���Y�����c�h6�t#v�`�.�U���s�{��X�Xax��. ��7�x�m�ͪ�0F�y���p%��.��۫��K�A�Dfb��W+h7ar�3?�D]`���=���I�$p&�Q��ZRk ZTE� ��H6�=F�8dm���I��>�&�d��A�>�L`������89	��[Z�MS�Wu��6�;��K��W�u]eB
��S��}V�m�P���7&���o$�f�hx��UMs�0��W0��C�`7��F��q�8qz�(���v���
�18x&>Hb?�>�j�{*$�����>�y�K�M+�,p��1)H����-�9�X�b��T -�I�dK޶�~ �س���sYlD}ִ�ga��HJ���WAye�R�)�GWӼ6o�$ɕEeкC�jz�*ƞ#a���wb�
��W��H�d%˹(Zp�'7��q%�ٞ	��4+�&
�Gx@_!�)��U��ȟ]P��ÔE���	)�@Y5 �d��B������96�&(�J�K]��s׫�B��gl��}Y�U��8w՛)Ď�c�{U�>6w��	P����ɡ���zXX��G��J��Q8� �.��m��w-�q���IR
�5{�~8��L}�Xճ.����D�)�Y�.��V�O�8��h��y��=��F^Z^���X�y��4�.��嘱&�k-aO�ԹAj���.�����U���C+�8D��0t�{6o���r����U�X��~���|��s��09�Y!v��hKŵT�փN�e��j�6�ު�$<�4
�,f$�e�D�x7�N#=�z�F�\>�1��[/�����#/,_�p�J�v����dZ�R���B�`2*hl�4gj$���J�u%iP�mf4��	��uW����	���?�(ܧ��!x���}ΣV�ZT���g��n�g���U��S��Zlť�P���X�bN4(��ԁQ}�kF��<��lFV�&�F���$&�2N���1�M���4U}�3����:�U��ꕩ��uYt�!r\�SX&�`��|�e## ��&V�x340031QHL�����())`��a�}�p��Y�������bdb 
�����əi�ɉ%��y���,��|+�Svn��7BLLI��MaXܕ"�����z�=q���(LAjAN~enj^	Ha��e;�s��پy����7��XWX���R�H����������z�Lu4ֆ+*�/()z�a����/���Le���c��1TQj^Hš9�%�ߘ���p����}�[n@U����d&�%悔~��j�i!6��:��-���G��ބ*�O,-ɀYೌIw��*m�4��;�I�{ AV�ZX�Y�
�%�A{m��GqN��>?��-/�8V q+����Zx��ϴ�yB�Ir~nn~�n~biI��nqjQYj�nZjbIiQ�^n
Ó;g��k�MU�#_=c��f���A�� /Dz��x��V�n�6��+( ��mV����� ���.�@�H��X&U>|�����![�\4�P��3s�8�s����Kv�.-Ǖ+2؋qs��r�`��]6{Z<�ZQ�~,8�.*o6ٝ�&���_3h�FT`��ŻQ2����$l�蚍��� ]��м���� �����\xd{���m��Jm�
�����5��4H[Z<<8RlJ�����J�|tI,57�Ҫ����T��j�]�SD��KQ�4������䲆�$���������\���K[��	<{9.>�C)�x���:�������u=T��`M����G%���w�mR%�%U�]��U��OG%Jl���Jׄ3�r����e�)~�ϰ{�%_�&V�����δZ���h{i�qm0g�j�[6�\�Є��Ђ�7��Rl�6݂h��v��O������
y�느I��A���O�<99tA���a镘�:��~�Ξ攮D�2&4U��Ң��EȜ�Cq��cB.�����E��xO?�r�~fI�~=�Hp���C��I�8,����o*+v< ��-l0����[��;`�����	�
Uō	rU�q]���]�Ҟ���ߺf�°���mB���ڎ�ǂj\�G�o��I?�؋AzL�D�l��N�<�bO�d	�&��=׵��e>�	�3̓���'�y�P}�c�U9응�耺����ق4Q�=��=*),-��DܸZX��V� �v'�Z��"�<�������X#l�,��^`� =�\L2�P�\
O�V��Ko`��k�E��-׍�ay�/clB3t/�-N�(��d��8��`�n0 ��j~��}�`a��yo�����A?��8��y{����v8��0�[�Z�a缡;mA�'VMh�Wc5~ �yr�N���Yi���)�cN§��:L�/�:�~〇��W���&h�;1H_�<�{m��A+���{?�<MZd�Y{�/3�����O�q�4_m�b�4�9��qc�7hD�:"��[l�#��;���x�340031QH,-��/ʬJ,��ϋO�OI��Ma�����w#���麟�����~mѐ�WR�_\��\R�誇}L�v�:a�O������s�9Ta>����ԔԼ��Ĝb���wL#��󲝕������e;!7������������`�>'���J�g���fHJOɄ*+JM+J-Έ/��N��ݗ�?y����=[$�͎�T� =V��x�E�=O�0���
�Kn���w̑�8P!�)I�t��$=�я���/��I<�]:�%�v�Z#O
x8̠z���ހR�h�Jgq�	r�S�y����~�N��G��pF�.H9����y���R-ik!6���&=1E�u6�uG����d�8r(	���g���~�h�����<H�S�����Lo-�j�[��N��ף}yn�3�-�[oh����]�  �^g��jx����0P�G�@G,9&1,'��?$8��б�#����a~w��|u1M����'܎�]]$3�$F�❬�[V�E8.���cwm��Yl���8;z\�������x��U�n�F}�W�Ƃ.�-��<�M�:@� v��0@-ɡ��j��]ZQ��{g��L�v�[�˙�9gFI�D���,?^�;��FjE��ǫ��VӇ�Ԉʯ��ƕ��U%m�'��B��W9ZEc ��a��*�4z8\��\8�A�����/g��{�R������|>�TP��͛UD�8o�eV��� ���1���C�$j�Ŝ�K��B�p�l�B�I���F{��ov%��DY�&fz�~���9
*��ť�)#r����W���F��3�97�.�n:�)Y`S�'� ���Q���\�0J���wPHT�[D�j���Q�o�F=X��yK��ޘxzg�&�:��ߐ�L$�ͯj�(t'i���f��G8��"�o�m��']��P�6�좚0H�u	���ʇ�q�����*�HO؁��b�pr7sY��%!a��XXt��t��W�%��[�T�#(�y�9O;G������r� ��]�J8g2)�c�k��:.~ �U赨T��=}����D���\fJ6,�l��X�Z+>�9;I�>i,��_9�ZlB.��v���hk~)�4{=���N��8�%�qvz�uZ��T���B�]"JY��slZ{_.�Se2�����<��:��K�HO�8��qV���<�N��D�c<)fG����VpW�� Ym�1
�>�.��
�65����)".��C�l#�`u]~���a��f�<�5=��L�p�h{mu�宗�:^nbďl.Y����w�ݫZ�>5
	��i	�m�R�%mP:x�	k���_ K�����������,�v۵�: ޜ��:��I�|3E��n8�ڄ�����Y�~X����D���1z2��T	Ū
���[�I-w-�F����{FNRJ�������g���NOPiI?;?���Z�;k��x��Ϟ=Y2�qဇcٖ����}7�~���\�8Ⱦ���B*�ƜC��c�2�զ*�,�y��4�x�-��� �w��0K�"�ফ�C�LN�h�F� ��wh����#Ԁz܅�F�|B��;!�d]Ǒ	��LU0)d�yͫZj�'?Soѩ>�e��J��N��m�:=��"���GJ� <�4<����hףYQ{jo��^R�?;�i<�,Zݖ&��Ax�x���R�M-.NLOU�RP
JM+J-�P(��N�S��/QH�/�KQ��RJ�O+110100 ��$���C%��p1�:ͩEE�E�,�� �*Ⱥ,x���OO�@���)&��	B��1FT ���:���N�݊��ݝi�у&�:�u������0FJ�pMyNVu�@��^��;��굡�WJ��ؔT�3\���&��O��I����>>=C#���0�&�b�$�?t^d�^R�TTh�7����^_,�s���nE�Ib�!��yj0K<��ݩ�R3�Y����MsS�r��L���^U�5�Ӓ�\.�J�,�A���.��1�8�M����V������VNs�}��ޘ����ۦ;�$U�$Uּ�!�̿ȳ���e�$�R�!�k��9�ЀLx}Q�j2�Շa�/O�|V�p���목z�E��R$:`2}�o�V4/]�%� sK�_ߛ	� ���2��x��X�o��|�W��|���f\�����p�SK)PFB�R+6��ɕ��}ߐ�K�|i�	,-�Ù73ouF7�p2I~�>Mi��z\Qn2o2*d%��2���5�Z�=�,eM�)KYy�J+�Z�=��:�J�?E�7��+���[��owKZ�^���gtw�\�^���m���?s�@��\�n��$��w���T����r�r�a��r��gXW����($��{���+���������n1��{��T�̼�
��o�#vd�b�M��4�/:�o�o�_'��rT[�/I�����H8R��U)lK�9o����r��J�K��D���҈�<I��hLDwjfrIVA�-�ާ8l����bQ(�m�s��(�@%�����)N������c�kG�i��2xP�Vd��l��.��RP��Q@CU���[ X�Ze���h�lDe�Kw焤L�>+��4<M��F���b��zQ
�]de><D+�����E�Y(�$�'b�D��:�.��� ���a��ۏ!Y�5U����J�uD�DK�3.�e�3Z�L�O�>�m"_jc}g�}� �'X�ϗO�pnol���-V�	6</��T<���V�Z��F�ڄdVR�m��C*�P'�J���:���;��%�&F�zL @d۰wr�Y�!
���k�:��R|��e�a�F;�tF+a`�}�+�zWϓ�H㱩|�z�4W���b�+���W�n/&8��9�0�3}���2ڟ{p�
/T�D7 �(��O��y�y��sv�����r�����
��߼��`��>/��\ ���eDذ��y�]:�[��T3O�nc��:#��$5�%�+)j��~w2T~Z@v���؄�� �vy2!�!$�n�_ʊ�}��xm�������e�>8�[S�!τ���_Ye�'C]uF��0`��0�v"���	���o��/߼��n�DrRK�/�9��&��n�X��H�	�d���F{f��5�c酪%��O�8D��ժ6*�����~;F %{�@��7����q����
�u��g���r��A������"^m4��S�qXg0A��� ����^��u[�輛yd�.o��f�$��_h�Ґ�@D����;d�\�lŎ��]jj~9��P8k	g���&5�Ϩ��d������� ��U�`Q�5���ʨ�dL����&ü.XK5�����٭���6	�5���J:5����2�A*u^�J�C��x|��{��Qip
��ơؔ��6��n��ܱtZ��Uj�{��)&U��\^��_6uAd��=x�SQė��3��RZ�9�y������aX9�u�"�(Y�|GMQ9����T�eai/�lz��>� 嬭3�s�џ����(ʹ��9��ȏ)I�m�����d#���҇���I}PRX>�Ӑ�8��{._(S�$ކ���7�l%b�;����y�p��9Hɉ�b�u�G��|AΠd��m�P�hlf><����rf�d��{q�+�iS�@����jm�u�;�_B"s�v�Y��@S��ڞ:0�c$47��/��aga�j#b�­���������rբ�p;�̡����C�7&��(���AP<*�\Ä�p�^�f�$?Ny,Pm��!8�Ñv��c8����$;�,�Tf��_,�9O� ��;ix·�P�ƅ����m�'�fJql���شn�A�7\\c�lô<V�_9���0�W�5Z��9%��Up�!6���x���$y��@�l���n�;W���b���t�i�w�~���aG� ��28�D�;E�z�&h�Bчj�L����Pg��I�(��-c��c����8���>&.���48�HHxl�x�=��K,;H���r��
R�&�n�/����oM�N��+�sϚ/����r��������*:�f�`�����������<�7���vx��T�o�6�_qC� ��moA1 �����d���DSg�5��Hʩ���xrR�H0��H���~�X��t�ѥ�x�CY¶��TАN��E�A%l���oQE��M}v�B"P������o����qq�����bUA��j��m��[_�j���7�,����c����/n�� %�{8���&a|���O�/����Y�)��1���*r�8N~E��2.�SN#�����>,׋�Յ��L�v0��=��H]����D�	}� ��Dd��U����@��h�=WC4�^�>`�[�=[͊����tq�z��K��������F�3�c��5����e�/?�<"��G��s�A�R?��֒C�( �D��5��Ѵ�O\���4&�ڋ�zϒ뢨�z�bW�&��ǔX�;�Ƹ}P��a�R9m]�������u�(~�����ao,�q�N͂X����ɏ#�-ie3F�1��P樂Q;�����6È��L�<�̭�ĝ��6��I5�Ĭ�'mGR�����a
��X�PQH��4�����R0��p?^��LqŁ1[���X�&��c��E�⌲�#Q�l�rO5����'^W�ϣp�CD)?r�����@�h�Ci;��)Ť�B�0=a����z2�� ���q����c�cug������uoXxu���t���(#3!j�'F"�y�r�Y�E�Y�D@v�.l�	Z�N+�X��`ĐA�C�ș�ْ� 6���AVp���	�!��T_�79�q�������2�|)�a'��D�	���}S��s�1=��Z�|#6�=��y��#���~���x��Z_oܸ��`������c����Ÿ�Nm��^�\���$�H���S�ϐ�~��$�͐���:qs�w������\�*�����/Jq}s|u#2�6&��������剨%�k�j���TU#V�*�!��l�l�3I�7�����qvz~#n^�_�볓��ˋ�8���9;>Wg����H��R�wz|s���G�'��M�N�*��lZ�����j���n��dv鴿.�j�MEG2^J��tx  ���=��S*����5`���v�T�uXI��P����
�_�:����FnM�`���vf���5x-t�cO�B�
�o-s+K߷&]�5�����_f��ZU�ܹV�#��K�9/��"}�\��qH�K ���0X����e7+�Y���
^��^!�,�/p�J���UZ���P�Hw� *z1t*��~��Ue
�kj���ϟ�/��qL.݋�y���d���qMn����\^.HN��K��ȯ��[�� �ԏr���T��\��5ke��U���B̈́KeA���'	��+���_��Me��Լ2�D�,JY��i[��)mFf�T^�Sƕ_�;dτH-�:�D���V:�$,��(��S�^��NV�,]������kF�hu�J|}s��z�m�BȺ.>:*v�7
6�T�����t�gUmlCw'}�]�+����8@�Z�L��
��ep�{I�Q�L��X�P�J%���U��Y��hk�9>>9?%�+'S�3-��͖\�=]�BZ(���	G���,yk�r3��'+��&�~��,MK<��mhJ�[Ո���R��!�S#jQ e[4�"�ж)�QdK�$��|w;��[�BW�U�&������S`7�<�<����w�	Yio�#F
�N@�2B��mҘ�@���{����Q��P��j�*��U7p<Ou�.P�nV��3%`9��A�h⼗@ "EHǽm��8�$��Բi��f�S��D ;�*�S�SJ]	"�Ά��-1��fgsA���V���,� o�@����Q�]���W�8�?1�s�@���Ūij���(�u:���y�j8����08�˷�9��RmMu�~-<�.����}�:����s}|1����������E{���m=C�r�TNi�[L��Ðt���A�ĞO��{l03G����5#�(�H*�7Y;��Y���s�����.�G�d���p�;D��߽b��q��5�)���C��K���r@�ڂk	��,M����_��#�=d)�g>�drXZ�,�>�)�����/��6�>�#�'�����8z��"��Q݉�f��������Hw-:�hVX�u,��{cv�j�p�؁9iTY��ù�'��@A?�&r�$�
ԵV�sb��P��`Du`(9����C�v8\�؊����;v��$�&G�I�.V�>�GV��\�JQy�:��xn���G7AO6�L(�fgEDN!WW>dq����5ѡ<S����8`sY�O�C5�cQ�%܃3�� B]$���`iFYȭ̦��Q
�V}2EswwGk�}��o�����ܫ+��_>�<=������:�|;u[�!E�F���l�a}|
K��`R>�C�|([L`�p�;ýF�����
�x��sD�BUݻ�U�X�e��h,�:޷?��p�c�
�+�.e1��/p[�Z���v��4:]W��96L�4f*2D��
��
�}lyN��)��T��-o��hN���Ύ�(�>�E!h�����7bȡ\��ΊY�j���vG~O��/f1]��4����G�����/�au�%#s�M7-�X��ਚ���x�*l�j&�6�d��&K�1��1�����E/e��Ǐ��-�b?���F��-n?��2�������G�����펥C�U�����I���q���{��"X{̣<�2��p��J�$�@�[Y�/uR=^GVEo�gr��՟U�c��&���`�\+*-va�o�O�.���_���=���H-�]Ȋw/ŉW�?��u�H\�R���W03$�tE�O���)K�F��<����p�h)a�ޯ6�F^�_tF|
�����)n�H�D�c�bL��A�Lʐ��|�SS��N�\�������&g�z�g!��C1Ɍ��`�1�[I4Z�t����z�)��憛��z�bsXJ�fjQ�-@���疴���j?��@be���@��=O;�?:;h��'�`Yx%N}�I�N��b:�)�F�3��g�-݀����ZU���0!���6F�^���W��ق�����rt�/�	;&KW�����55�{|4�q�<���ݱ�#�^RP���;Q��ͷ�T�K���5$
��cY�Hn�<o��m���a}0���V^�����q'��#��>j0qo!�.r�520 �|�6g�g3��Z�Ϥ�tN/�0f�$�dȤ͸���g�e��ÏЎjߊ��ˠ��- ���c��y��U�9���|�b�B�"bM#���G����x�$[,�JAC.� �ĸI���1\��g֩H,�.Z�j:I,FQ�:���%�d]�}q���2��'���Jy#EMZ�)�����I�ʆ���G�AiX��N�=}3h�+�P�X(Rl�z	��ʳk���9� !YF�9�Z���Č0ç�|�fw�{�Z*�"j:
�NX
�_�k����W�.t���{���Hz~;0��(���Z�qGcvT��}��l�͌��L�ݝ��#��!T��*����5KHr���I�ew����obd$Z0�9������<������Ӽ2�ߪ�AP }�&������d۬�1���<�R N�4�����!��nL�͗`���nt���,�B��Aܓ�j��J�<��	��]��V��3�m&��`�l⬙��.7���F����c;#j����� MJTN(��H ~����1���-�v9��y�ͦ�/_}6I�<�i+ �n���&������p�J�x���W=Hb`�!�D�h��6b�-g�S3�]���#�G�a���������цy�{vp���_����O��x6I��f�ޡod!n2
6���}�^ w�@{�?���bD�@g�k7�w�=���Ճ��1W+~?�dx-3n�i�(��\�~s��G'4�� ��h`68��kׂ�Oṵ� ##�w5SC�,���G���������g�=n�����g�[sm^��ME�x2�HA�sLD�#^�xC)OW�ł��FsX�N�i�e|�)?� ��9>�YS��~)��?��I���*�bm�ɱ���[i�����\zoBH.1X(X}��;�J/���G`F2%�S+7,�cz�L�b�Tc�/v�����(��JK�K!{
��	�:\���7��Λ6��>�#C��/GN.*<v��!�����,�_�^"�,��y)GԽ1�����2� ��$A��F;��X�}ذ���~�;.I���k�:�Ŏ�wvYq���^�<�e���6�qa�}e�K;a���>_���f�h�?�ʲڒ]�{�h/Ϗ������ȝ*�Z����;��3SQҝ|Ʒ+���k2�K��x���Qs�F�w}���m&j�&}H��,.2c	8����$]9@If�Ề#�T�4n����
H�J&1Or�gJ�Eĳ���cp��Pe�� V<��<jM �8�8<p�B ���5Wr	,��`�,�o����zE�2�@t��1\pɄ��ӥD��!c��|�I-��u��j�z	b�"��Zd��Hdy��o�y
ϰ.VdXn�4U�_俌F���A߱�g�i.d�{Րc�\��r�%+��wQQ�"f|��<+u,Za|Ͷ=b����m�^�]&��4�"�V�"ID���>a��؞�Y��L{��X.��Ԙ�����RU��W������d��rY���-�0&�*?��y{uu�,Yan(�OĔ���1�ZgK�H�cj3ojy�����F\-K�͹JXtjB�����cϼ4`���C۷w�������hp6�0vL�yd�ǻ��7�>a�\���\]6�"iT����G0)˲/R��?F��<ap.�E9Bψg�Ri��<N#��3f�}�&�i�ӿR���W1$E���r�3=Ip��(��y�����.�V M���H��#B�'��cf�B��>h�%�v����LNI�ug���I�t3�b^�i
EV~G=w���jn5��ЮgD��l�5������-X����r��әL��YS����=�{�H$W�7�mS���]3�(�mܶ<�)�D�]e�U���?�	�f��.+b��N�Ա��]�f�d�����������v�[�촪�,�|�*;#��A�������&���ĵ���Uc�o���_�M;�j��d��e*��ricr���jY��b{�O��G���	\Y(��,��Ox�ϕ��ͤ���y�*���+���x����(����Z�_P����\���W�X���7Q� �^��x��WMo1��W���U��F9�6%�����^��������������~�)��́%��f<��3h���ȇ[�W�o.b���E�#�F�k6
DƻS22�pfíYK%~r#d�f�v�
�)����V��".�]^&�HA�O�C��B���������U��cWW������"0�Llԛ��<�777C�L���'9f��"�S)�Z&�l4`X����S�}���9��n>p-|���%�wn�}�$
x0�+a��g����٪H3�����2��{N>aP!m�^�u9��lMDFIc�(�o<�-��(��~�va�S�ci��1щ�
���S����H��p��Ru*&�K�A���3���g)�ՠ"����?[I�w��:��e�}$���:;�M��jg{�	��N�M�+�Qi2��?<�¦8�*��Y��A90(�d�z�ov�����d�������n7	s��&Q��3�-�8��ɟ���E˫�6Uu��16"*�!T��G��4c��"�A�3��
6����AI��������;��Pco�6)���V��j��`4+�C�(����E�{G>ԹL�3Qf8�eE�U���y��c���!f��F��E�z<6�{�,�_ͱcr��� aM�����a|N�)lP�4�ۙ";U�$��f7��:��J�dǓ����L��F������F8+3���n�FrBx��K��q�,SقY�=��c>�G����}#vi���3;4��6��z�I:_4�����̭�=�о1y�ù>)S����E,Y��P;�w�g�#M��D�Eq=�]�����R&��Z��1�P)k��_v���;���}�C�Z,q�p}�u���-�DQ�P7���,�3O��o�^�d�+��Sx�ՔAk�0������{m��^04]hh�IH���bƣq�VkI�M��+�!�\JO�=��ӼX,�|�>��7�����u��8�Aw��M�$I��-c3Jb�"�����2�����ɫ/��}���Һ���O�!�v��g��k�Q�˪�����{��=І�0h��+�?��5v�VS�F��"o~R�*�$�2O����$JyS�B`����v���<^��z��A��\=]D��9�;j-������)�y�?&��B�'�@cg�3N�}]W�h�Z�"�X����Z&-ϳ�׍(��)l�
f�֒�}6I����!��)&���*O������X9��sx���l������c��s�>Q�=�WHl��K(p��'�tTL@�,q�⢠��(
����J`�H���Heÿ�>XT#��a�=�~�t�f?�����K�x�340031Q(-N-�M��+N�+)��Ma��d9�瓙�����b�r� v�q��x��V�n�6}�W��ڀ]�ݠ�"�zw��c/E�"p���%Q*I9�����(�%���g�93�����R[����J+]�PM���e���D�"��,�RK��^/�~P���V+i�k���j��
�cᄃ�揥��\���ts Q8����(�.$Q�X��8W��'�F�+A2���r�RJ�*�G!�a����g:�A-���a��\I%n��biD����(�RE��&4�(��m%�W�|ޕڙ2���6K+c��,���5��J �����`'�h΋e9�\�p�Q �G�,v��JG���z%�| �E�M�*�!������p2!�4�p��>�e�t���`Saj�!�"�jΑ@2JTp���>F�@Z(�D�PY��U��aՎ�5���R'�_�l��ʅ��S���0�JS(k9K�c&�icpN��Vݫ�����T�[C�Ȃ	�TZ�D�4P����""9*u�:�9^%������l��n1�?N�L�:Sg�68��}wkM~��I(�ԥ��lpv5�6��H%4�_��vry=�|�w���7jwn���f~uu5��̆���l��쟏��,ds&g����lp}���"uw�k��0�O�2����//n8B��d|>��o��O��cqǶ{˺Y�����D�UĕA�`�BC]Oe\�VP�k&��B�p����+�Y0�������N-Y�0c�<��k�6�T
.g������J�X���%V�\B��Q�(�U5�=~B	!�XrZ?���)H�V, ���Dq�ǭ"���^��ק�M����N�����޴*��2U�w��4j�{,�;����!��\�������H�vz�;}strrt����������ёu���m�w2;y���������1��i���2�=k�׃X��6� 8�CT����K�x7袡c�%ފLj���s��4�(���;eTA��
�U��~Yq\"?����(��n[R;�y�*\�o��<�bt;!n�ܲBȾ�I<`^��[r�`hdA.�z���9~�Jï3�s�F!�̸�Fg�0|11J�4�!!E_7��kr���(��}�zٞ"[��*�K��&�2)ͪ� AWů?G*���
�M�H�'9�5�J�734�vm�z%���q|���,�@X#���)��N<��qJ������G�N!Q�`���\��g��m�c�y�����{�ڶ��(�*�?��Bgr4�'�~��:��a�>����`ʛ���T�ǝ��34D2:�|��Fp�\��z�tz�?oz�^�nJ�Y��Gg}:W��A�ㄟǺSã�o�>M8?�oO��`
3v:L%���g����w7�z�+k�l{��i�,���j�7L�g�aӴ�l}5"Hh��f��ma��!���}K��bi\@aI���d�;P��\���OQ �w��ԖհO<=��;$���{��i�h��3����4yP=Z���vÆ�L����T{>��&�(:�/��S�k���o�?�t����.n����ϡ��W��]-�`6���
������w��������5]�5�Y�4Ag�j�5�c �`�S��i�	x�340031Q��ON��K�+c��}�h�����O�+��	��g�!DQAQ~JirIf~XeƎ������.��ϻ_���[�����T�����>�3���HU��`�kI�**I-.��9�_�ǯ畻x�<�,��� �=�&x�u�_k�0���)
{��}�m���(�)d5seiӵ�o��u��^�������M��B�R� �Q���� N��_c�<��>�����y3�u{t��q�ހ�O(U�����lk��.N�JY@aJ�юq��m�#~����HĒ�Vv_
��u�y>��bc�xb_��AJ/*Xz=�ʢ�#�����O��8ː�"�Q��ό;��$�V�E靔҄�8@޾�ě��b��)@W1�{��u�C��(c�l�Id��>,pR�;V��N�oș9���\��(���zY����`�(�>��h2L"�gtSÒ�6�@m���4�5�����Xx�{�r�m�t��ch�G����kpp|����_����k��������DA��[�k��
C#K�"0����f��crrjqqH~vj�OfZjIfn���H.YG��2�K����RӊR�3��B���4QHI�,�ZZ�ᜟ��d��TTC7J1 ��D`��x�;�v�m�kN~rb�F�L��}L
P�#&k(K�Y ���,x�����mB��WxH|��s�k�mqjrQj���+L �
���rx�;�v�m�KAQ~�F�L`��}L
�#&kC�Y H$��+x�;���mB��WxH|��s�k�mqjrQj�ƺ�L �

���Dx�;�v�m�(���BqIbz�F�L�`��}L
P�#&k(K�Y ����0x�����mB��WxH|��s�k�mqjrQj���+L ��
���x�;���mB��WxH|��s�k�mqjrQj�ƺ�L �

��x�31 ���܂���Ԋ���Ԣ���T�+sl�nK�-�RI���aY��� |�ԣx�31 ������<��M��?�����vLt�:K�?�&`�ũɉũ�]��w\����Jq��[�C�� s���x�340031Q(N�-�I�O�(�/N-*�LN�O��M���K�g�H��x������˿��Y��z `�F��x��WMs�6=��㓔���LO�9R�(iS��\Q�H.���N�{ )~�rbL
�}�a?@ ���#`�s�_b�e�y2�P6�&WS��ʛ$B̮"i�����<��=WG� ��D
���,�I�*�qఃЁ��
B���h���2R����8R���6��H}�&xxM�M�1��<�a��6� ���?:fQ������#R;�<s̀�lL����&�%;�Y.�������+"f�4���7�����L#�榦d�&�w��-��HHڝ֦���׏R�ӝ�r�*������SaYY'��x�(���Ѱ��7��E,)FCs&�m��3B�)������"�p�\3���R��9SJ�l֫���﷏R}��E�+G�ڂ౾U|L��A�_�� +�j�F��3 ]��0�P�F%��6��8�+�<���pǵ��*<C�k��	9e���KFc;z�g�kY�Uj���&��u���4�Y.�)��a$GH�p��TcԺJ��QwHc��ʡ�Z�-�����KR�iØ
��A�ۢ�`���泆�����\4V0�8�Q��Uj/�M�#� h���T_�Wh��/+n����.Y+�_x,�=��D�$kn�L<[ɿ����iX��9.J���QH0b�[��@]ތ6����b0���4�P|���0<kŶ���r�ݣ�Xe]O��"��F�~ݗ\-�����T���zZcL�(Ȱ/�l-g7ċ�;�����q�Y��MܔsG���@c�W'�~���G��dy��e7+
�m��S�(�=��tQb�F���M�:,�o]��ʄ��"���-�<�F��:]a��ǺȀ]��j��Q�q�]I�S:�U,�����������5�q@�QQ|�m�ސL��Z�G{�S梫�.��R���}�;$�U���f����w1����M�I�k���aT�O�T���Xx��,?��� 19;1=U!��$39'u"��4�RzfIFi�^r~�~z~~zN�~iif���I�m�9�8a��,��L����9���cy�-�b
ZP�KI��sQjbI�#D (��4��(�����j�^1f}�M�r�����BjQQ~��f^C���O�c �RV���!x��Ƚ�I� 19;1=U�81� 'յ�$8��,3y"��D�`	M.��ʂT�`T��ɊL�yY�'�b柼�ِ�� -}�x�340031Q(N�-�I�O�(�/N-*�LN�/-NMN,N�K�g�_65Ǻ��>GŊ�{�+r��~ V&�<x��R�N�0<�_���:���B �"�n�Eb��
�ǎCS('?43;;���Oj��U�Tx�������\y�ԍ#�\d�,�'Bd���
�a�6��>H��ri���QћZ��!��]���b,�nVMn��eʗ�(��ù�������;$��2I݇��Ob�S��lM�� r4����U�Ĉ*#>��'X!�5�y�gj5�G�1�Z�x��.>�X�V�%�������,�j���L�@�-Y8q;6O`�OE�����o�hD��9a��Yu�y�"�y:�@�G۝�?��ܢ����8�q
����Y]Q��������m�N�Х\ w
�Z�l�͋0���j�h��^�]��̪#��5U4��xNa���J��Av��H�y`1���:�����X���x�U�r�8}^�;������6 P�D$A��~�I���)W�*�����Q��h!iM<��]ǵ��������K;S{�F��"�v�K�9�w;�������.�OSc���w�7�=�B2e?��D)�<H!rqLO��uu<�%ȇ4ZMP�gJxK&����ou�.�jl�Z�]PB�B��8�u�TEEq*��l����Y8l�]��ӿtR�h��)g��gzdq��*�8�Z��h4��ձ7�* L��+V�wvH�v�� �#��i�ѽ���]ߘ�O٬�BT��C�i�"�
&�UIk*>�R��X���pi9x���S�jŻ
�rOϧ���V��e��r��*�?�⸆B�A�4�j.*E��
V:
ƏC��[�8.�(�{u��9�]�uZ�b}�<B���?��rl��3�[~U��B���o���q�y�Ks9����ü0%O'$vh�PS�5�ck�Z�8a�3*��>����ybz�+YǇ�?��/o��ߴO�=����X����],T0�]�O�S�[���@�Z[xS�8*z3mN�&P�<�sc��?���tΐ�«}�[��Uc\���n �-s�q���=i]�Ѣ��y�T�=�_;�<MB�?�f��0EGV)���j�=�X#Doz\���I�K!˽Huv��A�LW�jn�����o�;���Ӑ�4z���7#��{`��������v辳BϬ�h��m�o�ܢ��]��t�w+ߍ6I�Z�'�_�"����{T��L�[���M>/�C6�-2�?'{�藷o���`_��&z.�B��A����	����u��̷'�8��`�x|0����:�&�L�v�@�w�!ӏ�/E	ذ��tx�B �����:4�;�ecomerce v0.1.8������^79.0��P���!96���z<�]��Qx��#xGp�CFˉ�7�2�db4����- u�	W��x��ײ�H�.|�b�K��D�H� C��X�{���Z�����=��ѽ��/��YiKnZt޷gQ<S��[d��G�|èo�巬����׾�Vr���h;��-@��:DR�����M>�m��'�Ҩ,�oQ��`�^��$���G��ߐN<D;*��3�;��X�f=w��n�W�̭��+���q��g���_©����1�@6�@��^5h�>�V#���yFm�9�o�:oo]Fp[d�r|���&�����m��A^�ʾ�}K6�&iJ��)0�X����%@����j�Y��>0eQ�0�Q���h�sh�e}l���hEB$'dC��.���M;�N��T��t�Ĥ����:2�$;�G�5[.;��v��$��ޅuڲ�>��*���I���vyv[��<1�#8���!���z$J�뺯o���w�7mxQ�I)ԝ�X��h���Ю��	>E4�������ן��B�.?C�0�*�.�5M�*�y��
h�Jy?w�"̛"�ݴp�O5_AP�{731�n�5X�x2ȟ���O�TN��h�
���k>��i�.���n#@�(���?�u�~7�7}��%��"����T�9r���3�����]�xo޴��W7�����Gu��i���r��h䃒JAuT#6������G[gÝtڍ2U*:KX�2(��ܕ���8n3<�Qu��Qݚ�G(���	��?�
E��z�O# ΢�)��Ӏ�rS�������Go�}���ǧ�����߶��"ey��?����y����Ë�",B`��b=�#(&��E�Q(�}T���8����}�`Qt|\�`���/&�07��,e��&P��ev��?zl�+��ym{1�߮���&��z�h��o�MP��?g'׶�Zd�-ų{�}.��D5�BWi��\;Ea�̌�z<�n���}|k$�%�8���/��Ei����u����p��MϜZ58T'{�B��25���%�kĕ�SV�$� ~y��~Y������������G�$K�n8���˩�]�ɽ�E����>�6ް�ő䁎*�&�%���]W����J O�V�>�'j�6�J̩۝�^���B�1M��������<�mQ��W��r�OT��	��I�-r����Rn��-���E���waWL�٦��6��P3��O.G�:Z�a{��n���Yz������8-��y[��d�등oእ�0��l~H�>�<�ƀ�g��[��}sU��=��D�-�6�m�d���� ��jDL�k,�� U���g�/��r�g�fR����bw� ��+컵���2 �������K���ݲѝ�Jk�����ޞh(9a���G�t7i�7\-~���6�5�����gp�~{���������g�Sǝa0��>�CԻ�Mq�E��U���b���#ε�|BP��Ef���l�1Fː���Ay 7�F8{~���z�u]�����$�o� �@)���e������������WQ�Zݩ��m�L�/�r�l#S-E��D����z��K=����N��6y~�`6�Z8-���z�j��ݘ{��!*MV8���Z2�ǲ��R�%z�~75�	���~,��>�����t�x��g����kAD�};�У'��'9�\��?~�i����@�E9H@J�8��8٬�0���,8��#��3��;�2QW�;��u1z�7��=�wɬ>����d䶳۴Tv��#����B;XT֑r�ˆ�y�� ��J?�:���&�/��0�u\��B�[��������JyTo6������������~��by6��3;�0�� Uf.5�2�VCg������'�iZ�M��?�s���	��>MʮW��1�w�TR���"�qǐ������_�~]�,��n�=��.��"��x�?z*JWs���w�؜#�轌�~���Vǿ����=~����j]&�2�5�e̺Xuy��==�+,�6������E��o�O�'���;r������? �;�K|�ɠ�=0�3T|����$ ��x��p����a���s��#H������O��ט+a�U݊*r'�&�5��Y7ά���!MŞ@���4��t!��)�����Ia>0��§\�pVo�==�t�)=�ω�tܵ�H����y!%!�y�6.د�g�J�T�ЬN��ch/��C{����+��0q;c+�	A'C�z�E@o�b��Tz�hց�{��vr��*c�����J&��W��	����:�����4t��E�(g��������#�f�3�Zݗ��Y��1�=�h� 3�C+o��|���1�nV���W��rϰQ���"?b�Ű�Ji�E2ǘ6���+�I����հv�'�[��5�X�����N �����y�f�}��<2.N-v�~3���L�t�oy+�I�R���b-K5k.��>)�)����F'��#�����X	��V�v�1�ܡ���Ol��P���ds��m��v;�|"��rDvvpDG����(>��y^�
N��9��Aj���c|@�K0\���x�n���{��]��D_���V�ۤȦ4�ހ�̺�ĥ��L�Ǆ���k;�Ka�"������"oa�S�'��[���{��ј��s��yB��
O�C�>��/�\��XԤ���	�Y��om7��)�nW
,Wg�>kP�:4����gQGij�(�6�(�D�K���Y�d�$��mM�5E�dvh�G��E��D&�oA��թ�v��)๛��+�fA�ʹ�y��]bƝ,�y(���v���A���8�I)Y�."J��%yy�\�3M����D�%(���n�����A��e3#�X
S��uܶ!똷.A*C��̥S������t?�d���2 �x.��E�������'����g���8x�SnS�x�[|���o`D���3�7i���$���S�d�|�(�)��S����1C�[+��O��h�4�&�ATf�`\F]㷵��_E�����ؕJs��Z��L��^(�Т�k�J�u�3��})҈�g<�6~6s�	=L��8�"�Ā�؁"���T����� BWj�^��D�P�*ɖ�Ն��D�'?��\;�]j�����������A덵����f|R1��Y��)S�v�3U#�{�����������e���N��ԛ���_�����^7f��ӻ��ފ̀��5ia>�����\��<o��M6��A	�X���'~��S=ö@ ��D �����
ϧ��K9	�b�ώb��)ىh�{���[����]���}�ujJe�ҮT���u�<�ٸ=*�a{r`X�ީݻwN"�ɒ�������{�V��s*6<����pyG/�=�U�M	��%���_a��	j��+����{x�ϐ"���'�n��� h������ �W���O���J��hZx��@?���P�����Q��F���u7���(�����뽾m��]:��k�������g�i1}7_�%+���dS�P���G|*3�e�x�~�Y�+���O->XM��q$'�D�;�aF�END;�=۶�/Ɯ OT��H ���^14�YgQn��_uK��%$��j����F�Ǿ���e0��m�ifY��xa�y���["�A����Y2X$#��pBf�~l3U���v�ڄ
�+!����d6:%R�w��kŞ�Mx��) ���z��˒$'��-�Mq8��wo�@.�-�x}�R�Z9E���\^�Sg�*PDK�&ܵ�Kq�o~:n돯*�{}�88=�=����rt�u�Q?�Z}�##�3����O?���g�F�FĲ�Gea���)�:�N�^B�CŒ\��a�7�{w��+���|ٯi��$�uJy��å��)�5hU��f`R�Y�W>R�����.p�=����߃
f�"b'�Z�l�'���GK���X�2�@T������\�K�1m�0ܾ�V~]�1����T6���m��{����Ȳ���C6��4�t�P�2�YU�f>3f$���c@.Dآ�ʽ_�F\�zMuq4�� �|�VX�;ic��a�3BL�%����|����?���=�C���i�w�X.%ػs��S�Y�k�?�P����u���:Hqx��;tn�h�[��:�7� ��<���{0���:*�'r$B#	�5�='������Xg����],����nۗ3r�����軣@��@��1z�H����8*�e
=�5�@���a�]O�߃�ߣ�X�o,��!I��lUb�޵����^�J���_]ݱ��w�<j �/�|%�BN��Ӿ�O�Y`=���xi�.�8I
�)�&�~�U�]E��}�ӂ-m;ᘃ�5�  �(�@�ڍ���p��q����n���\1kBsI[MS�����Þ��ν���88�[��x���\w�?D^._n�����枔6��ߐ������Dr���1'�)M�����	u�����K�)
�Y)�N�x�,F�}�$^��.��w��ĲQ��/��nb�e��ռ�"�I�F�|�xm�������\%N����7|���y���aQ�J6���pL,x�z ō��>�1�dA$�~���.�+Y�!ZAbc��Ł�h�':Ã��GB�F}H�GD�З�� �K�DF��@�?�_u} �״��8�b,�~x哔M"��,��$<V1!�)Y+0!
��t�-d;"yO����MRV���1��\7E<B"�{�䊢��r[	��Skę��Qz����O�2����|5|���$zea+zr���\i 7�x^J�omzr�{j�\p�Ƌ�bﾬ�@`�p:?��o(�L9#p���Y��zGy����NR�."�����+Q��0�F\��|��~��t���@�Ӷń{�U��߅	ى�z�6�����@�]��uf���T�\غ�s�X[����(bФ�C�B8'�A�¿��b�1�a����rH�uwٌ!��k��m��c'[�A�<Wl�N�
d���7�ɀd,�K��s�.0p �|�zZ�'�$�_�����!��<��C�b(㉮�����&]x��Hꃼӆ��ɿ��	3�\ �Ë� \������f�
�i7]��ޑA3b�闂WL�܀nvh�p����R�t�pt��tR��5jG�t�<9&U���y���ɰ�F�Z���"����0�����`������PCI�:|�D�#�~麴;*Jr�~i��?�('v�L*���2�Gw���]�V��=�p�G���`���Խ>1�Nk+�A0{��<���H���JEƻ��m%���Z�^�R�$hF�������lm_w�EwG�]���i>�.��\< N�^�0=uLz�0�9#+3|z�D6����^⚨����_�����ä�zQT<C���gN�'�	�9�$Q�o��k��h�IA�.��P��9���@jr}�8DR�����G�uk�#o�$g���޵��B��K��$�]J���đPE�����d�j�uR�ra�>2;��8���<�~]�=�J6�\�ys��b}���3�cU~n����:��F1 =�]Ɂ~-�q`����s�F�~��#�(9��m��jk���d�K{�*��iu;�^7�����+���Hݝ�1D.q����x�-�7q<�Yh�]Uȣ�1�{��DuW������]� V���t�䵭�*�Io��w�j�&N��RO�,�������
ڽ�ׯ��*�p����l��2=�U�����t���ݯW���^��b]�x*Xi��Қ;_���Df4�FT���P�9Nm/��l��$܏G�Mp;(���m�k� �z>x�y>Ʉ}|�ߟ�V,�Qp;/�2ٲ�Hd��=.��d}ṃ��'<ٲ��X�-��~�����	2D��_Pb&�-`�~b���I-6t����w�3ڊEŽR~P��w�q���h�JВ�V�	�rl�E�ZP�q�.����z�����bQJ�z��%}n���s-0;�ډ#4��^E ��W����`A�`%��KH<aa��H7��F�Õ���{�`�:����2��F��m�7m�*G���B��s���C�a��m�3 �m:9zr���=��d��ƻ?�2�$V�
iJ�1T� ���}��#8�m��P�П�b��ү�ߥ���/�}u�Y��Q ��Ę1��1��&[�Kf_��o����3%VL��*	ʪ� ��mU5���,��!lq��F�ސ׊�'��7��S�z�GiӘ&�o1z��-����&�J<E��L��JΠ��J��=�כX|���J�j���I��/��uR��М��K���̕Z�XR�ʟ�Thg��p=ؤ��仱��S��,�ԗ�+0�7٠��L��R�.�Yz_�Ϯ�H���Z�Ê;��J�W���#���A�B׻���������p�0�#�e|b�h܂��0'�@��s'����{��?������KG��l��|�!��_A�-e��}��Q��ׇ �|�9��u��_�d�b����-�'l�)?�o=��E��}�*��c�C����'��I� �X�*{��o��t3ܕ�ǝ�V��4b�NB7�^ڻ�S!�]��[�/Ú�S�����a�c&���*���;ڠV�F�[��W|�6nm���?����NJ䆈��*X�P�
���j�g<7�!�t}�2�����7¬9:9!���<R*mՓ�D"zs���Q>�������.���kwY�OO��r�\!Ŧ�53�ȇ�L*̮�8/�-ԗF��0W�f�]�I�0�|�b����ιV]k�����gW��M��=`��h�p�^�i�O������y�V7ܣSDQ�;� �+�=ieh����8o��O�`�k���]��+\j�e�%�.#���-����' ��Lx��D�������:ǯ���9Y�.x�#�Dt	�p�F=D�
��r��G=�|��=�S;,&�2��O��k����%mc������'%����nvs5{]�յ��^#��D��c`G|�,�N�A�Q6�T�Mzf��P�%Q�{�vɢ~���_�Df��� ��`SNz�{\bo`������Of�N9K��̺�]�K���G�\���|Gp��S�G%rT7��0��%�V|�{�FK����		�4��+6<�"˅]�t���J݁ΐ.���,��o�Vg���Fr5!'������D�msMj���V��A�h��9#�^���U��t��X��o��V-��v�ѫ�-
��^����pP����%�v�G^oYZ�J�)A�S�e�E{\8�:��K*�k��)w�e�v5��A%�e)�i��:Ȑ΄߼pW`�/��ͽ�bE����Z�W��BbJ�̇6pJX{fz���
5
������ :�PR�UOv&S�^�H�\ί)��RC��X3 q�=�MT�$�5EOQg�j?[���Y�];���1�˦���r��5A(�4�`���/��E��r
��%��ܐ�R&t���~vG��n��w��Ib(�G���`
�	l���~����}�Q՝��#9y����ܨ�p��?�z� !�y��_RC��i�=��VG�o��`m��<�a���)ȥ�>'�`���9�Cq��]bFJ2�w��\yC�G����̌]{���ય���c�N�l�o͍0Ҋ���s�:���8�p�Rn��#q�NĭY�)֐�����KMAAml� l�;~�0����ۼ)I�L9�$
X�4v���c�8rW�W��ĽkRi���蟴�eeql���R��Qw}1醗o�]F�{��[Z��]���+�S	�8��_�-�0�	|ٸ���A�,��\?,������6�n?6
i�Wr���;�ȑS�������sWJ�-,�8�1�͂"�C9��E�Ld���Eŧ��ӧn����Ȱ�+���3}�UA�䷔`�:,
6�i{��2��c��NkY��i��A�M�r|�����T� %�G�
`pѳ�B������p��.���A��!P[����rI��0�q��H�H=o�ǢƉ���D�1��+����� f�l�!���YF���vߌ�8n.�s�&q|�<�K5�'Yz��n���֐�oƀ���>$NѸO `-#W2����}�'�{FQ���C�͆�9��q`v�_�b�e>m@`�(���3Kg���)R��P��$��P$�R��WH���cp�s��1��Bi
\C+R-�ߵ�?�F�װ����J�J7�c���d�e�T.~����.���1`��C:�C�f��ϑ{�����e�;A���2����I^54M˸E��,�;B��rF�;����`6��j�B�cp��:����b�+� �"8h�ɀ��Yzm�M�d�~L�G��>X���n�9�
��v���nc������ͮ��S���`?|l�8B�o�E�(��@'���l�l���<MM�6�n���e2��V-��#W 0�p��FI�[� PY�g�7����cn����D��޷�}P�E
6I������=M����lŝ�=(�ѽ�?j�4yG�[�~��T��&�ͧ�RwB���H�a�"~by�z��n�i;�D���[�̲n�I�v�:�<����u�R��%�ګ'���ЧB�����6�z}�G�Ĺ&z�>��a�r潫nK�\��A�D�!5�A0��o\���ذ���v#���8&g�E��w��C�,1��]�ݵ!�� ��C��`8��/�J�����O0�S�4���s��8�vv�\�;���kNM����!_�5S��-l��蚠>�Z*�'~�}{8@��?��a�;���@v~\�Fz��h�74r �,`�R�X������ J��3��IY?R��h-�!�0� �
�4O|���[}�?H<��2 }@>\��X�&p�Z�?ElzǍR�c � ��1Zy�|8,N��[����z��7x�m�Rk|q��|,ΐ>���H�M�0�~;��'���a�I|��;��9��ŋ�h����l��H-	��{\⠨�ٶˬ��?G� �2ѱ4�;�8��A��R��)6]�����*4&=����m{J���DYxx`t_�꿿�� �( A�%q׳=�f��}��H�($�|h��<�si�Y^/�����0@87
¡Y6p}j��ԩP��  ��AQ�ϸ4½uzX\��D�Cҹ���MH(����j�+O���y���~w�+�5���l����u� ����Et� � �>q���(�V�?G
l�-��xU�ql����J-� ��a	޼r� �:d�k�s���4���6p\l�,c�#�������� ��p������I�� K�WL���u᪛鱗���؋����b��̃�_���tZ1z�('_���
�5
�|3�24Y���Y`(�+��'�~\m���b����(����ӷ�W�������ۈ�y�$��k���H��lv�d�~��j�Bk)����4�n@.����A�x����t�ά��ߝ���
x������>�Z��+�Q���Nz���	N��]���|��BI�]I3_ˎ?x��xi�=�D}v�]��8���y�[�M,��`���,�h��eM.��N��Ohm����5tr� ��ܵp�t
��������z4O��Fݺ��;��f������u(���W��[�V:)M�(sĵ��;��3YtG.9�M�v3RE�}�����өRA?�(���*ńJ���| /Jq9�[z����3��כ�o��(\���GÜ���(K�`[�TT�ٙeƭ�M�{����?�[�D;�`j~B�����(pgJᏝ[�'�:H����{����@5䀊JVG�v�6g6`u�X�^�(�R�}pe������
�U�Ql����	���)�m9���:n���϶�]~�H��������Q)ܝ[&�}Ii��|��c�~&���.k�%MmҘg�����N}R��ɃWq2f����_�����.���q	)��+BQMS���ޱX5��)���p���}?ꂴ�eB� �)������F�|ۓD�^�.�x����k��MW@�u�ρ_Y8+h��(<IC��
��tI��e��s�Ƙv�|to�O�@+��M97�����k��y�FiE�&�,�Gt�cL`p�'#��f��	aA	[��`�������Ɯ>��n�\�G����lM�,S����c�/�z��~��/�Xpo��ԃ�'X�D�V'���@�Ċ �� ��||�d�1�J�1�y�Gb���.�;[��{��iNu��c���~�����Ae�ee)a�,h�Ѹ�y�H�gc���~��+^�*�Fn&��i���(R�ԯY���H0�����a.b���D�����>c�͞��#���&�v�@w=*����c#��̸�#��-������kP����ׁ0��8+�w�BM��t��H����E� �������}U�%7�@�d|���/
���#��w��3�OG~�.AS��芘���Qp�E�P�����H!1�p?�#�yv�Xb��oz?(؂d�k0��D��Y��qc�I��=�kQ;�N�Fg�?_�3���0�K�M�N�S��):�AL�[��iw��9x�:��3ؒ\��|�@�1�{8b���O�T�=��׹��O�ks�H�����9�������,r���q���ո1$�"���T�@i���B���n�`r�X�Z#{��?��r�]������Z��|	2k�S_��_�{� �Y�q��AP��1������^M;�ĩ���)>I�#�?H�2�)�~��J
n��J�Cr��6��>�{i�hyW��� 
9g���p�z��S��4�9x���!=�e;��;���;���t*���:��JG��!3�x��7u�Ѱ�!�B�*��<����V@]h	r�z)E14��[F�N�i,8y�E�-?���D}������A�TB��d�nƿA�]<(H�X��`D`Ȟ\r��fNY�{��f+t�ğ��]4���`ۣAm��=��ѿI��>�4�`|�4\Qk�$(�g��am/�f����H,Ʊ��T��N{�o����.�����.�����	
t�<�F�0f���F��L����5{�J�|�Z�G�9.���/r��['����ʿ��`q�_��Mw�AbL*[��]�S���*��G�i��xx���{��U�$�#�cX�:�M�jk~��,����f�������'B���`�_F��)�D����3=���}s���vg|u�~�+ޛYI��#H��YHˑP�D#>�Dǩ*�e�܍��F�7~��幣�!|I+'�s��6-F��T̇����v�������W��[^���ר�2�{���+���
,��`�������q)��������ZM!��"(25sW�s��.ӱJ^�]��1Q^���\�VLBz��C|�*G��}�O�J����]�u)�|��˝������"T<��X
�մ#Bvy�
-P$oH�f�T�>+9,P��qf�W|�� W̲G���@2�P�o�����JEj�<|IG}� Xp�&�_/�|��:
:���1`n���>.$d�l������9gKIU2a�����?@y:.�u��B`����O�^��˾7)��A���&�h��4��6�*ɉѠ���ǳO�rj�'���|�� �:j�+xl~��Nd%��ف&;K�}��}
:�X���*-j6"мq1���>��i�������^���7���ￇ[�F�5w�b_tyˌ�A�-������h63�r����MS���r��k��Z��=@P�NB���̆0��ӥ���K�ۂ��s����<�Mh����%��2�~��!6�(ʷY�����覄��/\�� �"� <�z�����G��	��+��-�%�A8 ���H��)��I� �zW5����^I��]��ol���<f��5�p!����S:k���y��O��N�����/Y�?�c��d(tW�Ux�#6�|Qp0C.�<�/��Qap��������A��/����bI�|VD�	zn�Sb�?S�<�u�x���ɪ�T�qZ�AQ�A7��+A
+s*%�b�JeNI�(7�P�J*S%��C�\��}�~�p#�퍗��K�k#�9p�w��#�������/�>s�$v3�}R�}��?	����f&��IjW:�����B�]��`W���N���G��^<x��� ~ҏ�È
4�zN�Y���X�ga�z�j�3��pz���N��Um�1J��ٱ�Z!p�����q��T����ז����5x�QD�Th��P�T��Xs���șdk��bNR2��<isK���._<��?n?��I��>�Ax:��tGXc��B1A�T@*�/*�Gd�������"(��U�U�W7�����(�6��c>#S��.89�Ał9���
���ټp�a���@]�\�����{�%��1L�y�!�X4d+2�۹���Yf,'@UP��-7�3����� �Z�MW�(�g.��ˋ~~����8&��+ܭ�E���H�v],���Ur���
����t��	�Z�jZ�Sg���|�����Y��!T?n�c�m� s�B&���z-noGF`n��n���c��$��fd�s�U���]��z�XA�&�|j�a��=�!|N#�M��T����4&��^�B�"{�=�Og��ow�@���t��eu-�i-9��8՛��f0i���21}а�N]9����z�X���U��@�
,ͮ>=���џ� �s˚`�T��_3�pԵ$���fSy\�6��a�j�B�
Q��z�s�V�_co��G>�w����`!�*)[b���~B�`�7u�°�ѮQD'��/2������y�1� s���{�2[�&k����*q��vq�M�������f��2l̤b��	���ynZ�3d>��� y�����k_��D�XaO9l],z�,��h)d��I�ѰMG�����χ��k�&r<�(~�;~:J�|�*&���h�� �9p2X���w���_|�����}�(0�li�0=oV�eF�/Ȕ����f����?{u����o���p[��$x�{q���5�b��
�V���eιy��A>�e�NEE>�9��9��!�UN����F��A������&'���[����槀���f����DXj�X&Wyd�&�e��T{%��{TU@�sN~�����S6��dB�q�e�kIR��AR�_YPIr~v��q�{�OXfinpdExe�W���c�d�t��Ҹ�6�N{�
 ��E2�.x��R�n�0|�+"���-���\�7�b������6i}�|��w��B>���p�DG#e�r�7��֘���%�
Q�����Ø	ΐZ����̡��a�ܶ�U�ɖ�k�Oqb�
f�"nم�%N�_%~��gj�X;�����T�R�]���>���+�Ƶ4�5��k0p�sQ�ў�K�)B==�cr�Ժ�����t��5!�8�H��;Vt���u���Ƅ��cq4�v�{��<뗆�(����:�+����~m<���n���p4_�#�}��F'����lCy�����o�!ՅE��_�B�y�	x�31 �Ģ���T�>��)��>�Y�-��j�~��Ғ��<��Ē��<�FyZS�3������LcF��b��Ĝ������l��ϱ�������<��Ux����|��'N_�s*pqKQ���$���7_] ��>���x���8�q������l��>�S�(�\n���ۭ�����Q�x�31 �������ҢĒ�"��&��b�k}Z.qH���������d��U2��ft�g���ja޷�n��Ӄ*��M��c8xEq�������52���8��^UP�ϰ�١�g~m>S9#v_���a;BdS+�SJ2��'$�;	6/Y�R����)��+6�!j���4�G���M=��rA���%�٢Ԃ��L�?*���^�8�~�c�i��4�����
QT�Z\R�0w�թ�~�lO��,�}��-���/-NMN,Ne�?譭s��`�K{�c�W�m/�M ��~(�x�340031QH,*�L�I�O��K�L/-J,�/�K�ghؽ��)��<�=��M��\Q�= ��Ȳ�x����o�0ǟ����J�6�x��+� 4�+׽�����2��8�V:}h����ޝ�.%;���ngZmd�[�Q$��F��h4ZY���hd��,���. 豈/f� ��n�*z���NN�|-�\O�R��Z Kn!#��EV���_���9�p$����R�OuQ ���TA�c@Y)��Zek(��!�	T�[[�'�-���G���[�k'^Xd��Y�A��s�^D�"�Jm$���%������{XI�����tʡ�+��R=jo��*O��J�Hݝ�JS՗(��3��Z�m�媺O �6�}R���� ��/Es%Qd%0�kbf,:a�����Rz�.�(B gÀҫ�6z���S�}�����i���)?a]�}
�u�ث~�R\0)z
��u7&��f	k�{� ���0@�E<�xo؉a�RR�?�E�z��w����h�L
�9^�YQ��:���r���]̻���ݮ������L]]�!�QA���I����T��,c~�F�|8̃�pħu9�]�I�{8�C.E�����I눚����!�u�2N&l�Op��h�������V�+���S�a��e,W�4�+�w/)�[n�q����T�CϦ��j�n<$'m��~H!V�u�h���m�����@�����";՜�:�]�:���zԀO:Ͻ��r���5�dA��϶!p��Hx��$�(�_��������XT����:�G�ʜ��I��$5� '�$ur �0TB��P�k�nT�%�DJjNfYjQ�~zQA�'T�GII��r�2��U�3��FN��(�a���&f�M`4�i� ��cr�����g��U��'ڙL>���UQZ���X��ٕ9�j��9,0���60�3VSS�-ƬgӐ�0���R�3�KR����َ��u�k��[ك�����g�p� �?�خx�31 ����d��U�����,.m��\���<��	X:����!!����[¢y>s�2����q#D:;1-;�ᔜ��Ɵ��r�Z�4������ �p$ɦx�340031QH,*�L�I�O/*H�O��+)���I-�K�g8qC�+��yN������tl~"  J�Yx��TMo�0=�_1ˡ�����ݤ�r�aW��5j05C�h���c �Ou�rތ�{�!�J��A92��{W녭�٢@'�)k�"��a�P(�;�6/P�PU.��Ӝ��f؄��)j�0�=�@�zi���e��W3{Qn�2��Β}n�3².a��Yi��2Ф�7{�K[*S]&>�1\��T1�eE8Q��ى�n��&c�gw,����m����AC��E�6�P�8�9�Gŧ�����أV�O����m�j�u\MV�y�C�|�����)3��C��g�i���trѿp����ȃ��ņb�.g4��L ���������Y��e���J.dx؍� W,���t#�wt�,���|R�ɘ�@�Q,�����-T���v����������8'DL{�q麊�	-�iȓn'��/}p�����f����lH����C�Ty'cKw��~N�^w}Oo���#~mW�-�ᑳ�t�r�J�]��:y�Qw1�Ǌ��KdYae	��0���6�èx�340031QH,*�L�I��())�O��+)���I-�K�gp����W��Q;^�e�8=o�%�l�EWQ~i	D��A3��y˳v����'���� q�*���x��Z�n�8][_�zQH�#mf�A��4L� N:��0��	K���������Ro[r?
�M�D]��9��$�d��8N��B�d�c%xRaY,J�Pȶz���yz�y>[��9߰ϼ��D�~�h��XQ��xƂ��3*G<�,F���bEE�C���|��P���4�Y����E�wѨx�Q�I�'�Lx,��̀� �^�2�K��ҥ�(�s��}˱,�-����BR��(���K%b�W5+�{��Y?,k��}��U�픴	;��&<���*1zY9��(%����h�����K����rA5��D��z�N_�"[�v�������v�^ p�nt"�d��O:,>��.��d�}Ǌ�xJ4"����g��}0#�R�s�k��q���J����jJ����Ҩ��a5Z�t���O��O��l�d����y�)Y 5�Ȥ �t\��� ���T)���u��R�l�=�"��Ô�}#3}��c3�BO�<i���7���~�_pȲ�A�b穀���t3]L�};󩨨��`�_��l�����@��D��U�z��e�yܠ�11���A��P�`?�J#=}[_k��o��nl�z����]4�H�k�}G�9��/��ӊ@���{׊"��v��8�9����
dq����錆��D{�aL2�V��~�Q������Ko;�L@�|��>��[�ʱnVF;���@�f����=�OV����i�#���*O�jfM���t�r�a`��cF���$�>�&J���CoW��,�8Rh(&�'���K����i���VmǤ�����2�ghf7� ������m̩����,I���@�f?k&�f3�X����GE���"��F��قY��Ϥ��:��V�?q�zJk�e�7���R�}&U���ъ�_�;����>U6Y�rR�	c��J�e�Փ��U9ٙ#-�]"����7��� c�I�;�P�*T��B3�#t�((�5��ZP�K͒����5R� u�,�ѥ������������h�z��]��Mh9�Nl�r>��n�!`Re�U��npj�k�a}o�#�j�#jh"�Z�ʂ>�ǎ�!1�~ܜ�8(�@r�lX(qV�{Ճ5'͆�3�we����͌��>:>�W���s.�����U�zЍ���3�;}���j��RƟ���v��mR��� ���U�z�~�RϨ����t^�+,����V�~�7`�N��botl̎�JN��wG��M	�?�-���{x�Օ�KQ��4kG�M3��K�L���JQI��X�P�24�n3���xg���?����Q��Kԛ�/B��;3�nn����̽3��=�s�p>d֭gf0�1^x{�6NA�f����*�
S��*�_�ἘX�!'�W��Q�����Ff�
��� [,%��^�0��f=6/{k���RxΣ���\LhY��H�&'y>D0��	`��/,#e �j�}h|\ݗ�t%3 �x�a*n��ĭ[FTe��͡JS�Vީ>�.1Lc'̻�GqI���'t��A�
������eeA�����s�To���k�pxbt�,Jo�rQ�Z���u�8���U]���u't谄~�Y>����	uL��\��zt@�P�[�d*cU-1-"4�6MK�(�Z�����7��I��OGkt��i�ҕ"F�)���R0(�B6��^���Ӳ����ݻ��m��xT�?hi�+���|�c��Kn��~��3����cQԹm�������k�g6���ԕ��n�|/��u؀]���������ߌs��4�k��]C�݊�Ì*n��7����}o8qvG�����Ɠr���͊���<��<��xǟe�yM�%� f��.Hr��.�� �j�'�x�����?4ԏ�'[�T#��G+*��`/ψXF0Wu%Ph��L���&<��,l��@;�,-�%r����6P��?x�{�y��� 19;1=U!��$39'u"���6vo��4�����������	p)ɟ��T.]������h3c�$FC8���L?�<���T���TM�&s1��:�&��:BE
4$��Jp��$]+�SJ2���`��|�hf	���a(GU�Mb2�����d.	6�D�4'o�r�kr�r �t��(x����j�0���S�FR��Sa��m��t�����&�Q�2������nF�~�'KN���4H���[�e¦�$�iC"��2\�m��-��
�j,�c!"�3��X��-��d���,MQa�E��n]#YƲ&+�"�ڲQ�ڢ�b�
�G�a��o���+���p�O>pb筂w}8;$��R���}i�d��\�w����":]�X��֕�B��Y�X�J�]���x}�%�8Q�	�|�����i4��@�-4����~UK[��ɮ; �Sl9ȫQ��;�
�O�e����+��f��m���fTӰ�~k�r)�j��@x�ke��,^�����������S�ᜑ���QRR0QJ�Ih��5���O�E�>f���W��������V�Q(�K��+)���I-�+��� ��/��cx�����Y� 19;1=U!��$39'գ���9?��(?''�h��;Tb��5�9U�<kcN�fl���P҇
)�(�%�M�s.JM,Iu�Hjr�r =7.ئx�31 ��������"����7����N�X��^pU��	XIAQ~Ji2P����ɣ3���s�,�6Z�R�q ��լx�340031QH��+.�M-�K�g��=a�<�i{�Z��u���~�e��E�E�M��v��^=�=�e�ɓYOBE���_x��TQo�0~ƿ�C�mb�=��˒n�&�!�4i/�k� �4���w6���Q��	|��w�}��)+h�1�F���YAJ6m�5B���68BAȔ4����T��0���0U%�(�tC���"�մL���U]R�Z�2�I�(Bþ�)���&q�y8��VP�<p�i];���Z��i���|M��M�m�o<��8S[�����W{���k.�7��H#d�5���qct�~D�4��BŊ�r#IE��V2|�w}*:����Zd�F�Ss�j�?��<>b���e��E��=W1󀽥ɢ{ƶC�0#V��O���)��.��][ap�k��l>nҍN��
R����n�+D��I�̂Tã��E�ͱ�����/Z�u�S�ę7�BgC̠R���ܯ�I�0�
��ۭ������<�0�Vm=�����d&r���ۥe붰 [ُ�h�#f��¸O۔7$�܊2����	F����ё2� �%���i�:���<@���#�`�����
������tʀ��0����Xi��@=�o9*�nxɻ0�p|= Y*	�Ϭ��8���p�׻9���E$�>x����n�0���O1D*JPֹ/�
T�
�J	q�:����cg���w�?��RBJ����y�̌B�D� �)��hw�6�Ncj9��*�F;�u��E-M�tW��Fg����7�@�)��n�4Cը�Zo݉FU�YJ�Y;�^8���}5�W�3�7�ogLo��t�<��p�`����\�9�V�$�Y�ݣv��|���So��*Y3����.힭�D<��f�x���w���9���B�����dW����l�-	���Z��	�z����ҙ�v������+�Ϲ���tr������J�1�����(/DlЎ,��x�]����	��^���cXG��@(!�3�����ѕi[���U|�����/?tVF���_��e��!z�>FL�[G~.� _�]�H�*�	�\ĚD�Yo��C�L1�����p��&��맞��0���٭�Jx�����	Q:�qvu���y�����о�uO��d��{dG�U @ͧx�340031Q((�O)MN-�K�g��^�>>����I>wԃ�����< �w�#x�u�1O�0���W�2��"��Z����\S+ql�/����iH%�l�����.h��A[���7zK����O�Xe�w�'�D�ז�N�T��a�֫f�Z�~PLn��i��R_�ƖMg]YU��YC�dt�Ռ�&u�U���n�X5�d�[~��mè0�3Q��0W 2���"V�0�?�&2��0r�����3����)?��-�wy7�G�=u�0'�,�Op���� �YU��ߵ667��g�yn��Z�f<�a�i������m��D~�!�%[nZ�\�>�?q`�z&�"��mP�­x�340031QH,*�L�I�O��M���K�gX���_�^����1�2&��� t�\�x�340031QH.JM,I�O,*�L�I�O)��K�gx�����E�U��|?-��oѱ�� Σ��*x��Q�N�0<�_��%(M�P/�� *�gI��@���nB�T�*����xgw�c���L[�,��T���bJ>Y#*f���v�^2�ڼV3�^�<<��"��(��T�`"�Pj�:����u/�����`�v��7%�E�w���|3J^E�#QII��k��=F������I1ga������qB�H� j���h�NK�:��}zڬ�k���^e��ʻG����M���	k��^�emW�e
�y����3���H�b>J$��C	�NI�ӈ�
�V�7[.}k���]L��� G�6�_K��h�x�340031QH,*�L�I�O�HN-(����K�g�#���R�[��MJ_���9 �*u�-x����N�0���S���$w�hU	$��T����骎]�u�Sߝ8.%�B�+c����BnD܊@����-�5�a�����e��uw��5�:<��6��^��{
��V�JZ�I�:$}��%s����6�(����e��� ���i~�D�
�V�H>s ���q�ܣ�4�"-��e;s~5�?���a���e���ń��!s@��Vdٞ����u췼5�=?*�Y�ۮ�f� ��s������c>gV��vP������)��������s�~�����e��3�T�Gx	�)%9�����)M[{���t���=x������� 19;1=U!��$39'u"�KvGs�o!F(�)3/%3/ݵ"YCS!��(�H����g�.S1L?L!H��oq�Lb�&) aD)��x�340031Q��O�K�g��)��dwcc�6���/)]�����<�(;����c󙻌�����z"{���� ۱�Mx������0���S�9$��@۴�)��Ma��b˲G2��lv�w�H���Rh	X���7��Ɲ(�BI�o��5!z�Y灒,/������ʲ~-�N�G��o�5+�;���⥳�އm ~�;������Y#�IT�+;�i�y��Z�%ט���.�ץ��U$�Y/X���vϏNt�t�L���Y�S�����e4O�=�!�>��[Yڃt����u	p�^a�����3�d�R�j��
����S'a�{������ �IX�$k�Rhp�����=y&��M	?��kE��`���<"�?@�c4��;��hP����Vw�z�u����h�h�.D9hQLIV$s����[,�)��RMa�6O��o�t����y��i� }	l1�E��a|�pB[�E�T6l���F_���?Z���aS����n����wQHCѯ��8�T/f��8���O���Y����j
��o�����`�4��w0~y�F���Gva
F�E"�U0�+�ű��mM�T3�щ����)$�߿ �
����x���y��� 19;1=U!��$39'u"�/;�g�am\~�&���)�	 �9��x�M�A�� E��),VP���H��\`քJBq�M;U�>�fэ�/}�}/���;4����z�p]�3Jh����=(�����ҵ���3��/�[��fYP �,�	�
#������X*���l�V�vy�u]�|��?^�-1��֡h�u�M�|S-:fbU�5�.h_0_i$)�k,��*���Blk�f���.]��x�340031QH,*�L�I�/J-��K�g�9še���a�u�&~�6گ E\ȽBx����o�0ǟ�_qEU1Z+�m�)�Ķ$�s�1�*��j�*��΄��M���Ͼ��}�6u*��BBjQ�Rndm��SUm,B�<_�r�>-�>���J��P��~��Tq����)��4Sqa���QVu���)Y��q�g��?I��?=4n�?)�@c�&&L,�l|2��Z�ѡA�
�7�i0V�K�`G��VH��d[@V�w�C8s�O����ZW�o��8���-�5̦��$nO�tY����?}#�I�
0�L�g����i?b!�kj�w�����P1��,�q�lW�������@��� ����Fet�n?=��\~���:��j��I��GPY�<2gSӖ��u�l�G��/�~yǿ�9zꀜ����J:~�t�}��e橼ӽX�V�cn���������E.�.LzA���'���A�|��P*�I�D��Й빹��V�:����z�������3B:���i���W�\����/q۪x�31 ��̊�Ң�b�ݳ�dN�V!�M��}���ϗ~&`%�y%��E�%��y�:�g},u}Zl0���ع}�,�� ?}��x�340031QH,*�L�I���+IM/J,��ϋOˬ()-J�K�g��#�"坘�M�C�]��wW�  qQi��x��VKo�6>K��5Ђ
�)=��a#'���Eg��^F�e"�(PT�t���R�,?R4��ę����7�x��sAo��JnL�E�u��!4F�*�ؘ���Y���ȵ�a���i!�>#�\�Us�R��3� OW\?�LƕVF�7�S#�U���su���V�J?�!x�T^����9S:�s]��%q�E&J#yQǲ�E
���nDmb��+1��C�rc�B?�T|�E�kq<���JB������C��  [�k��@˲�Fc�2�F�2�D!�~�ű�S�+�
 =�'�|�7ZeM*��zy@d���݊J��8�4J?{�7��1�~w}\h��`�c��F��{� 2���e�Zq[cY.5_�^�6����sT���jC.�^�g�/�99�����>��<W�� �\��ఴ�@j��Ԑap'�����t�eS�4
��l��ײK�Tx��⠊�\T�0ž"�;��f�k|t�+)$��h����d;q��.�"�gJ#BO��Dh�t�U��
��L�	2"���h�>f���ƙSdO��o ��T)5�1I]�����pڂR�x4�kՔ�����}��G�����m�����0�w���(�0f	jB"��F�&ע�X`�r��mU-)��2��K��sR���<Qh5,Qb	�
<8�����:<Y��r�b#��Z�P>8;2�����vQ�Nk�j���s��}>E|�Neʬ𛖐' ��Y#;��m'B3�a`��9m����{�1٭Ř��~�c�� [�a�A#�~学bɛ�|�YV�'n�&��V5��ueu��Z\�-�A7]C�W����v�F-6��VE�UY��<{m�mJ[=�D�V��|[��a��<���Ӓ�ғK����RN����E$�a [!k���ٰ��CI[���jYڞ�����O̾Pp��{�7[�w^�y�sT��K:r�p���3����sB~~ٓ��X�b
y��ZO �̦p��T�g(#H/��nՐC[5���mҠ�;���q��2���l��cHۺ�L�|�i^�xeN�[(��P�
[�ھ���ސ���~�::,(��E���x�P�p�l��v�A7N�炸-���P�/V:�tWA,�p�Op�ä� &�W���z0��P�w�������W���oy��!x��'�-�_��������XT������7���v/*H���l��ꧤ�d��U�%��8��<JJ
@��Еe %�&09�A�L��0�LV�P�E��ř%�E�H�~1�B�mva�eR�L�Q��K+Jt�/��QH-*R��U�t��+I��K-��K-�t�|��rr�g�5�p�>v8�=�38��,���#%�qHn.��e ��o��x�340031QH.JM,I�O,*�L�I�/I-.�K�g��i��˶{e\$�j��� �_���x��XKo�8>[����B*	� ���&n7���ni,s+�*E�q�����(�mo@E.�D�|3����<��3`\�D�0�2͍Pr��<1+�6,�:~�����d�R!���RI:�`�1����c�K�Q��G:�g��:hU$9���L�i�%j��8�r�䩈��z�&�fE�ę:��D�Չ���	)珥�ƐLU<�c�fϨ�FwM���<om�F�-؟��T���RƓ���"�k��a��F�����b�G5��ר�iیfL2��)1YƼ,A��TY	VH�,�(S9�Y�tf���4t���̲ Fj#Ra�T%�}�:"��^����s��Rw�={ޤ�	,{��l�*�s��|�0��m�b�� �:bb�~�dR�֩�٨�ƣqZp�G�ܵ��\��i���uO-��(Q#�>���$���TyW���qU߸N�E�Uxt�%�iU���Ur
�BH��-ߨ�G�"3�3�`�5�����L��^+փ9� M(F��B�r����:p��f�h�w8c�\�Ɣ9��2C���4~F�4l���*�!��P�ߊ�J�bv2ń���"���k��L��s����#ɲ�l{)Ym�jt�u	������AM��xN�H"m��5�l\鬚�]Z����pS�A��b:���K������&�쟤���A��������'�i��[��wY|3z7��|�R���f���Z�g���-nlLM����N���g��h�=�.%�Ew`�*�W%^ߏy!pOk��sn3��x
:X��lD��k��<�uνS�0�c\(0�t~7��_E.K~�6^v}M�&����ȉ�.'h�����9؄��@�)id?܎���j��r�0��&û�k}���U�iWE�%,�������w-sw&���_r����#��k�.��%��җ���Ƭ�;�Oj�f�V��Q�JB��RVO�o�x_��z��ݘ{0�Un�D�������䚧.{����i°�����+�?@��Dר��g��;��q�W�����5�5�=;�(�{�w��_�<^��W��a}4�A��P��Ҧ.�]4����B��*�&������/
s�d�x�340031QH,*�L�I�/-NMN,N�K�g�dg�f��fҲ���b'��*��� z���^x��TQO�0~�ŭS�����	1����r�k�5����V�}�ą�R�i�K�������욋5/�qR�xo�[dLV�6b��V�nD���Υ*�_V��H!ݪY�BW�ŢB���|��B�˫����ݡy�/tť�~a.�r��f�s�zRIa��!r��F�2���?�X��hD-��{t�z@8�t�����0�v5B��֙F8��"����i���3�޾d��u���h�V\���p�'�n{һ���3c�F	�������/��?P=fɛ�����1
>��P��c:������\7��I�h~��bq�8	�87H�u֩���B8��y��G8y�tPr��ZG���l���c@c�I�W!�����Z�Pk�$,�˶��)(YB�pk:��X^.���__\�B���W��/�3c泭��n�"���9�=#ICv�&6�J+n�а��F�������ւ[�����
t�%�J�Y�@�����_��Kz)�(v�]ʢ9��� �i���Υ�ݐ�u_��-�nT'�n|�+��.d��%����s�s���Q�G�/��䦗����f�$|��x�31 �������ҢĒ�"�WK~%:��]�Qr־�3*sM��RRs2�R�*fI�5mμS��<��ȡ7W���"������<��M���W�T�-/��pj��|'TAI>���'�\;�-}���+a���ljErjAIf~�rU�!��_�:���x �j�����$�G����M���?5�܅YV���lQjA~q&�����}�O=��/F�T�Ԭ�<~_�����b[�d�/���k���훜��4{)�|iqjrbq*������\.���~�����7M �2~�x�340031QH,-ɈO��K�L/-J,�/�K�gH.>��UF{&��q�}���W�� ���x�31 ����d��s'#�'�\\V�}V�E�,�QRR������k�v��Z���T}��4�tvbZv"Â����>�J�&�k����< A,$̣x�340031QH,-ɈO/*H�O��+)���I-�K�g藭P���۳�Ȳr��k��U��N��x��WKO�0>7���%�M�����Ҿ��2δ����qxl�߱��)�K*�"����y~3��f��������ʌE��H���癐��^o����.'\M�됉y�L�|�1�&b0�L���y�P��m��c1�<%[�s�)S�D�>�����wC�bo�\��ST�%���l6�$�Hs�R�w'�I��u1 �d`����Aނ4^K�ۏ;^�y�!�ꮒ\ɂ)��zEC�����	/��{��q�2���l�r�"\"G�*dJv�I���~@
��z�uD����f@�М�#�?L8��gꞔ��O�9�I��*y:	�����z��R������h�(�[BpY:�zl�N	˔��l��������!Iy�A�P�m�TMO���4����S���*4v�������.�$���~��~7䪆�ul�m ף�+�J
w~ŝ�NQ����rH���&�����LZ��ݝ3��E�ÅW(�R��2?�� zwS%�GS����5#g�*�7w�϶�{&���5#��]��s�����hVh����-<xO�F����c��b�!e�aC�S5���Pe�TK���}ƑK�Ԩ�u�i߯�@k �V�Y��QwZfT7����L@=0B�?Tq���L\oצ�K�#77�q�T��a?��d�R�U.�iڊ؉���|�n������xBex�ib�B�v.aK����vD�b�i@��#��۵����H�a\d~���ى��y�n����F��Q�:p�?��������.��,�˟:���6�}�cP���.���mg����Kvq��ݑ�-�6�רt$o��ʮ��ӸiЉ��2��O��-7k������y�O�.�w�yτ�/׿��F��w�~x����n�@�eaZ;�¥�PL����'#�V�ZE�
�8��3u,�l�]S$ԇ{��RG<�	\� ��G��	��;�����ǭ�7�>r^!��^�-��v�G=Q����Ϯda���K(8c�_����yWT�_�.��eף���r�Q����B=�����OEk��
>C��RR+q6a �Uü�'a��j���N[�Y_]Q��lM�D���ҕ��_��ݻ��P���5|߰��@R�Y�u����/�Iq��N/V
���9��?<����a>~�:�H�a%&^��6����)t����N�]]�蠞O!��o�o˥EIA��\!��ٱ�6�-�m"��Q7 Ӈ��
G�f�����86�u�|~�>�����%�9��>ľ�)7ep�	]c�vZ��zegZ�ǌ�RDb�=Y�Iĸ��s1���R�c#;v�Q�`���"5���L�J���mRX�M�ςM���Mn�ck�ۈB��fb%ؤ\[v��������e-Mޜݒ�烛�F�bd|N�O��p�V������vx���E�t[��.nC����(��g�LL����i�⹈��NJ� ������Ux��!�[~�!WbiI�K~nbf��FF��k���8'vŰ�%J�!�ړ�Mc�A��-NuN,N�T��4{��T��Q,o�K3S�@�(5=��$�H/4��Eg�3���'X
.�����29�Si�<�	���?0mV�ϴ�׃Q����3�$����J!-1�8U(2ك���N��8d��T&��Ks!$K�JAr�:
�EE\�\ J�S��x�340031QH,-Ɉ�())�O��+)���I-�K�g��3�۽a��u���{'&���+`���(����������|9�ϼgm�^Ӣ H�(R��xx���~E�� 19;1=Ua"���h��Ғ�Լ���Ē����<�E����lv`�fܼO�3/ �
��x�31 ��������"���&Ww	^�s^�S$f�r9�I�M�J
��SJ��J��4V��y=����i��Zݿ� �M��x�340031QH��+.�M-�K�g��L\1El���I��~{���C��!DYy~Q6DѢ���jf=6?̸�[�
Ζ�  ]����=x������� 19;1=U!��$c"�*k>�am��R�J2�K2��6�
1A�6OcJe �RQ��#x�{����� 19;1=U!��$c"�k>��c��R�J2�K2��6v70A�&1� n���x�340031Q((�O)MN-�K�g�Z#�8��F�Y�Cݲ�͐  ����]x��²��� 19;1=U!��$c"�;k>�k��R�J2�K2��&�����x H���x�340031QH,-ɈO��M���K�g8�X2�+s��R�,3���5�51 �������W�2�2���{��ݲ�co�Y �?����4x��,La�f��Դ��⌐��Լ�	��X�6�c�����^��W☜�Z\<��Uz������]ٵ&�a3�\�*����� ����[x�;�pQaC��PV��U�g8 7�	�"x���pF}C+;'gPjzfqIj����b�)%�
Z���%.%�z�ũEA�����%@����TqA~^q��BjQQ~�&�sFb^zj@bqqy~Q
��8TQ@�F1]�n�E��%�VĢZ�� ��*Э-HI,I�0/1w�D6�ɓ9�'?����˾n�d6��XY';p�M6�Қ�Ǯ4y+G$�-:Y��p�=g�d3 g%���;�N��y'��r�O�Ξ4y%���m�3X'�s�M�Q�P�y
6���r23P�#��پ�W�~�+�u$r8���f�L������)b����Ѣ,��@���y�Y4�K*�3 J����Ax�;�>Mc�!���Ғ���R�E�%�I�i������Eɩ����Ԣ��"0�	�/3����Y����re���B-��^@bqqy~Q�{Qb^IPjaijq���V����5;���̪Ē��<���T�`W7�"���tfm#�s2S�J��RS�TfbN1#�������ٸ�Դ��⌐���<&a(�|S�3# T8�k��Gx���qC�� 19;1=Ua"g�\biIFj^IfrbIf~�~J~nbf�~n~Jj���*t%�@��R$�0É�Ri�*�,��k�a6e)��KI�^hqjQPjaijq	�����5���}r�'X,��8����i��vm����	h���AVK4���g���iĮd2��~4�Ɂ�Rb�Z�3��S'�c��<�]��]�[~Qz~I@bqqy~Q
n�'?gw��n@hAJbI*(l�sS�U�de!��3��q��o��x��!�Tl�f~�Ԓ����"�J99�E����t���Um�=�_l l�~�	x�340031QH��L�+�K�g`P�Ԡ��j��\�{�<�w(B������|�<�#�؆)�K>]p|o��TQQ~N*H���ϯ>�y������EW/9aURZ�ZR�s�í]�^Uݏ{����Y �=��x�uλ�0й�
+ ;kYC+f��V	ͣ$�P!���$<���H�dO�e���[�������,[i�F �4�F�c��b|AQzk���Ln�[��ol�� �>�w��@Q�
��E���)��:H�:a�lwI�ԋ�]X�[
z�?�����O���!x������� 19;1=Ua"� 7Xr�%x�ő=o� �g�+�����w�����`�:(�C��ZU���+JZ�sx��'�YL�Q:�h���i���cM;i:���hV�4�*��HHh�=c��o��|!$��<���t��tpT�c�Y�� Ԛx*���V�,� (y�Sx����0C��b�T!���y^Hw��#m���=���0
���pY������\��Ny������r.��"��Y��K���_R^�����i�:���:vi���Ļ��}x���r��� 19;1=Ua#�? 9D��x�]�1�0E��V���]X�IS+54q�8Bܝ�R�������G(&�ԉ7 �GYk�:塱�['�fjs��G$<�L�t�V�	ձ�ϭZ��%��á.�%	]�:O�Zpp+mj(�.o\�]8*K���o��~�2�����Gx�Z�L��x�{˸��� 19;1=Ua"���0����<!����%��\�\ �R��x�m���0���)����҅!3u���8�G!ޝZX�ɧ�}穻P��)�S͖���sH($%j)�E���^�!�1�����Vl-;�vlԺF��w����SЮ�F�K���sd��ͧ�E�rAU���*�>��c8q�~ m)��wߏ�T����/�V���&x�kfjb�(HL�NLOU��� )���x�340031QH�H�KO�/H,..�/J�K�g8���ү�h���^տ�X��BT���痠����qpF�(+/�c�ɒcA>ʧ����K�K�S�@���>�4y�CM�&�����o۫	UW���Y\�Z�R�Rz��ns����5W�9[��z-��*--HI,I�/-N-�K�M��V�%t����������<����
 �*U���Hx� ������[hE�h��B�ιڟ�:kM𳑯.�4�Mx��S�n�0=G_!�P�CbذC�):؀b]{�"��VY�$�k;��'*ucg)V`�����G�쥺���2b{����z��6�}�Z[>o�qS)�DmnͲ��A�F4���D��V�ơ�NZA�ڡQ8QC'�����NZSK?�Nr���~�'JJN�)�E��>@��L	�Zw��dz���7��2�����G�y@�_lvy��ӗB�5��{ ����l6$��S8Q���D�x��7G�����3�m�x���jix��y\�|z��;~r�����*w�d�D뮤��صR.e�ZR��!��Z��Q�泥�O�e�I/C��Z�y�eMZ���$YQ�dq���:�c{�dm]�V���"pF��Bd{;u�d���^�AO�Q�\{�nho����!�"�����3��d�$��y��8d�x�g]�%�|�/I��k�->,���� _�L�l��=���� ��C�c������bx���y�{�6�ļ�ԉ�'�1*1)(L�g����K-H,..�/JQ(.)��KWH�*�ϳR�K-��l�(�8Y�I
Ƭf��1�2JØ�e'�`T��QV�h+��
=���ܰĜ�T��L�X�c���Q )(@u���VKCJK3St�89L?H�1Ȇ%k�j���J�4��v�dZ�
a������h* ����X�
U������2�@+&����\6�� ��@�@x�u��k�0Ɵ���jX��� {���H�F�X����T�ώ[2��,��ﻓ�,YVCp8���>���BVF�n~��i�u��(.ӫRn�?s�\T���z��r�[^�"­[��Y��R5t�����j�+�9�X��"�١�|��R�
�;�GqO��*2�(�M���ƵB�X~,�}��,��N;����M�X�>�H���uj��ᗻ��K����t�[t_q�uL!	2@k�Myo�+� ��ꎲ	W&���q�nn��ɻ���Y��Ƚ���Wb1�������dO��{�_�V�n�;�R�xp��?ؾ�c�.m��G��6/��$(`���3�S��A)sd��i�*a��Y�nHd�NS��8N?���u��V5�m��ۖI�k��b.�C�F�fg�97(�������2�6boy�r�SD��+�Ȝ���<�k �T5A8-g�*���M�˷�p�C�txtP�=I�Ϸ�;� +īdJ/}�������]���x������(HL�NLOU���� 0�Zx��TMo�0=G��ȡ����u�\t(�E��ڪ6�h�EO���C��D�v�5�����{��B�*EP��m	�󂬃@̈c�)��v��>�)��~�˝��*�2�e�cKK�y�)�R�֨L�m4N��i22�\i#s曋٣�t�������|,[����c�ʪ��$����yG����s
��m��TX:oJg���/1�Q9rH�dV�����J��O��R�a�\S���	���f�^�o��!H��XK�]h̒�A잀�Gk�|r�����*k�l����
<QtA6�QY���Q�����wU�_+ol�1j���v'e]���m��tO=�ؗYjK���+�[�L'��Q�Q�닶�ˍ�zؔh]�j��,xMy��u��3��b����XqC�d�4���� ��/�AK�tQ��R�A�ȕ�zf�9���v���h��$�	M�v�����/)|��j0���?,��i����B�!�X���(2%BE�/������H�{�-�r��x�[�}�{�3�L����e�+$d��Y)��f�(%Lv`����( 6���Lx��S�n�0=G_!�P�Cb��C�-:�.��s�s�Z�'QE�!�>Q�;K�[9�=>�G���TR�^
�M��d&fȵ�XC+獦m����z�U�Yպlpit�pI`�V��8�ڒ����i�e�Fi[֛�٣ju��D;�������Sbs�OYشP����UO1Q	����s�A��u�`�ٛ�/�#�'�Ʌ��O1[�?^K�Œ�����Ѯ�i�}�����);!�[ɬ�o����m�xVѓd����=E�y�适���,!�)�*%���:s�� Y%_0eߵ��2�a��"�z<q!�tu�)\Ҟd�������v�}�۳`����������N1�u���� GX_bpƖ�!lx7�W�xu�z��O� Dn�Kp.����ᖊ���u/�C����2�u���)#���~�?����Ĩ�	lC��r!߽������!���z��-)
^n���|*���,�ͭx�340031QH,-ɈO�HN-(����K�gp~�w'���^+�� �����V�{l�:��x��VQo�0~ƿ��	$Hޑx(��H��`�e�*/9�Ebg�C�M��;;	-�m�r�ww�ww6)V,*Yf��� Rå �'�T��H���׆6#n��O/����.���B�G���@ɮ�$��?]E~ �6L�yȯO1:j�F�i#��5�s���rO�Yd97<���<��:h�6!��f
���u=3�dz(C����i���ȅ�k5��J(�ì�v�4�e&����1��{*_t�BG/)WP ����+m6�X���9DH`��q;<d
;�3�sJ��W���Tu��SX(��orbW�w�Bw�:)�{���	!� n`G��;`rµ�"z[�Z�z�S��;
=�&@�cZ�,֙:����T�{zg�+���_���~�hfT�N.u�ۦ�u����+��������h�]�ϻ�)A-�j��X�!��Z�r[.|%�%W�[�[�w�ݙpK���n�Ac��r(�-v>���;&��ŷ�M��T��z(S�@0{��޸�[�W�m,{��F�������޺�^o���7]5��&o�4�"�ִ"U�qgY��.;����أĶM��Eͩ��99�]����,��<�RsGfs�+�_x�,�6��8�եtC���6��&�It)�ǔ��)�u�}��[m��j�~�jq��x�{#�!�Q�����������q�b6���X�q {[�x�340031Q��O�K�gX(�>�k��xTA�b�i�}��$�!*��S�@�������%,�vg�w�� B���#x���4��� 19;1=U!��$c#?# K���x�340031QH,-Ɉ/J-��K�g��U�������#ޟ8�-���!D]rNfj^	\eU�O�W�t�_�6(�L�8A�#��(?'�D}�������+n}�|}p:TIqr~X��$����+�k�=vS�M>*����8���8�����O�k��
�(�zrk2 S�J���*x��ø�q�
�ooWoS/�םlWй�{�{�D ��O�x�u��n�@D��WX9 ���瞫�?X6&�ȮW�WU@�{5DH�m3of�'��/6�)��&:p̢�[����>$zNX�lC9�A������u��^��A�1�y�F���&?��u@տ��������^i�a`s&��0�LK0�@���mQ�ׯ�+���������I��_.�k�^�dEn�����w������xZ��Ex�;�t�I� 19;1=U!��$#(� �8�$����+3� ��DA��3$5Q˚D��d&'�d��M̚�
����h C��x��Umo�0���
/�P���n�����-ZG��n[/q�"��������K-l�V��ܝ��s��"���)�Q����d����jŅB��9�ݨ ��O"i$�K�N��B9?�a5
�L-��8�UT�%;[�%����b��g�V��(1@5)#s>*@T������g��K5+�G*��N5P������"t�Q�b!�%���ru���@c�;4��-O�:��\�;��-ZR?��6 �����<�{��"Pn�فۓ�dF��~�ik=�h2AJ{
ʿk��b�NEG|����c^U�s�An��~\n�d�� A&h����v�Qx�7آlj}��t��xA�%bwm���$�-�&��=M�yLW�!���PZ���f�e<�!��Ez��!K��S����iBO�o}2ҧ�
C'�\P���r3��?R��^%��vm r 3����8�҈Z�y��w�0�m����H߇`�S���:ЮԂ
G�>o���OD.�Q(\)�\��5�=�U����E�6���ϱ�������D�[�u�,,<7�VIk.V��#��L�8����#/�x;���F0��LoqC ������8C��� ���������xv�N��G�q��
�p�� ���\�>�E>�Zc�^/��d�^�����nc�^��-<���׳��?���;�O���k*�n>u���~d�� ��xhF!3aWbڔe���:ڱoh�(jxe�����-"��2'���N5%�؍@�nJ4��+xt�i,�����%�wϙc˽��ٺn�qi��{=��B��G�=����=���|Q�1w�N��O�|�j���a���߅����^X��0
�tBD�d����.�|�MN3ZB��^[�A�yl����������L��x�{ �*�Q�������0��t��=_biIFj^IfrbIf~��^߉+�8��R���S'�6�<�]l�S���첓�,'�b��u��+I�(�H.��Q�����	ӡ�9��9�,�X�����<=�L��\�������yV
�XmQ�ؼ�͒h��0���ɪz\*�J\��EE
V���D�SKPQ�a瞙� Rak�P\���ZT��_^� ��Ң<����k�j ��嗸�� U�N�a��\��ί�����U� �lS�;x�eS�n�0=[_��`��]��9At)��Y�[�-9��4���iӛ,�=>�G����H��5�*g�$��Cŵtr+-�����D�,�FPiz�4čr����Z�լ�t��*3�UEf�:�P�!i�#��G��752}*ģ�з(`A�6��[�k��\D��s��+<&q(�6v������Jסt}�-�3fo� ����	�V2��-(�	�%R���J9��*� �#l����
����L?p%��l�I!�z�b^d�<)�=(q�4�k1���#��ء����<��8���ŏ9ě����oPuZ�L\��HN����j���b��gl��ﱘ��ʻ���3�c+���h}瀣4gf�+'��h�r�TD<^�Ґ�=�a�|s���|��6��u��U����&�k~[f0�V�Dӹ<ϕN�oX-��xm��"��`�GN��%f�9�����.��9���wme��aD��������<<��%Vv�v���f`=�(-��cP��Đ��֣�╈��`:�,�XI=���x��;��� 19;1=Ua"��xr~^IjE��RJbIbRbq�~qa�D+g��Ғ�Լ���Ē��<������<�������ڼ���E���yũJ�L�b�M.�P�����u&�1jN��(3�'���P&G^��P5Pv:�:�&�d~f-N�yz�-�M%�8k���b%�Ey
E�9�:
y�9\�\ AB��x��U�n�@}��bX!�#^Ay�Mi�"��ړd���{�3{q.V<�qfϜ�sf��J,Dg�Sl��V�4��ViY��R5-�ׅ���(�zTɕ<[
�$*9Z���,�:��nkaqԮ#��U�Aw��ZMx��U6�R���|@;+U�`�
и��X���r�s����c�r���n�%B�s���a�>�Q����E-+���)!�-��m'��WV�G��(��3�a�&Yx��V���4!ZS�v�l,��!2֙&0�4c7	= �D�"�n�Y��s�\O���X2rV�	�}{�s�A[x܉�v�������y�R� �)|���(c9���h}�x�<P���e�V��%ޣ9�޹V�ʠ3�h� �/��.���C�y<�{��\�6�L��M��C�oI������k�}9����|��L'�_=�^��1���x�����]1�ԏ����|9X�!�Һ�/�N"� �rE�Vc�2,E�����`i�"oC*��9�����V3���^辞?C��d�b܍�,��>�g�t��R4�+����� C����}ڎw��A�-6U~��:x�֣�5Ƌ�]�;��uM�^1+.b�t�V�xVa-�r7�}��Uܑ�G%�-��)؛G(�X���%�p�������Ew��oy���S�����p	���6l��?��v�5;�G<j�����I�^J\��B�ӭ��z3�Pc�'΃����h[��C�a~ȟ$7{����T\[%2y��鞎���p��,��8�N/U�XZ��=�1�S�p0��0����uwt\�Y�6�tU쵅��h	�Ⱦ�x��Wmo�6�,����uV��k��C0�u�:�_֏)#�6�t)*n0��HJ�%ٛ�n��x���-I�ɚ� ����VL	���,�
�`�{AJ�#_� �R
Y�o�\��{!r�8k�6�]��|��{v�!�l�g9K�8S4�fD��JN����&�()5��h{��8��XgtT�,�;����MƊ�4�C� R7�G�oHQ�L�(_��t�8- g��e�j2��{}�1��h����B��F�E���"��&���e���Qx(��?�� $A�
�_C��e��a�Ȳ�r*�{Q��%�Z���hJw�@o
Vz3pq.��X��]�u�C{��:ze|B89�*η�Q��$�.�����
��j3s��ZX�	NM�"���8T'�C����V�f�Д��&FBBb��� �l�̄�/�����5���N�����P&�#~�v Iv0�t�J��@n�t��<p���B��k�H�C[g�tn0$�T'�ς�%����U��,�9&�0g��c
�6uz�;�����y|O�ҡ�����՛��(�-Qh�M���~v��8��C<����s<s��xS/^U/�I�l�&s�3�b��W<2���r!�j-iͿf�.��L3���+�h!�Ď�&�0�'�kEx�עˋ!�oKW����n�a����b�N9w.h�0EN64��B^#x;�ޗ 0	5��塙�p<�B�T̴޶��+%βaW||����0�&=���| <�P��#�]Z:i�6�I���d������<��y��%˯Sߍ3��}�=�抠��p6����e�7��tV8d#��Ң���^h�����7���./Ρ������P
�q����H�0Z.&v�I�g٣#�L���k9-��Ø�<?N`mVS�,�œ%���4�Ö8�40�6�� S�>�9�B��CyP����yn��г;�5��WG���ZV�U��B�ԜgS��Kl��Z�>,��~
a�pr�Z���bD�dx��&�k�4�n>�i��z9]^�����M	6�BҪSŮu��&'c�н4A�4¢�f6�?��c��foC4.؋>'X�0+�#� VYbm_���.��x����
�Aw̸��W�������2�����!�x�����?�66�x��M/��3�0�_����
�ۨEn^��Ig����sǳ�ؙU���N�F��482�WǪ&����뜟�FKN��v2�ވ0�@�p�TUe�Z�8Q���UV%Wxξ������;���Qgan�9�s{�3����k�lys�n�s��x�D�Z�z���gj����UK~�u2����n-b[I�=���=�ٺ�N�a|Ek�v���8����?�8�{��m��tn�|�ϸgQ���>�4�!���3�b��uMu*G��������B�#�0x�ۧܡ�Q�������0�SZ<9?�$��D��S)%�$1)�8U��0g��3_biIFj^IfrbIf~��N߉1�<���E���yũ�M��,�",�\R� 5K�B�(L�a1����t��/TT�(� �<yk�d}6N�yz���2�&r��0Q��i�dvF$Mu@/9x�D�7sr�1���U�P��2�"���_�����[Z�Z�0ف]dr �.�ݲh����E�׆�&!S�̩̑g��\'�#��&��O`S
H,..�/J��nlif�d^y��2[[%%�j.N�%%�Ey
��E�v�L^��;��@1LԘA# �Q�<��Gx��P����C��K__�-�$9#�8�ȩDz�(��e���+�d�*��*�t|��՜j�Z&�2J1�jFm���e��R
nA��`m�
��A�
�)
�
*�J�ŘB�@�KS�*u�j<]4����4�@<��2��ҙi
�EE
��
Ņ9z�EE~�A���
�\��y�5�8k'p��p\Z�6FG!/3��� !yG��x�31 ��̊�Ң�b�S��U\n�&m�)��l��
ug��̼����Ē���b���m��>�C��F.�O�E�8 =8�x�340031QH,-Ɉ��+IM/J,��ϋOˬ()-J�K�g�7a���z��Q��-z��fWRs 2� �x�340031QH.JM,I�O,-Ɉ/I-.�K�g��i��˶{e\$�j��� ��W�x�340031QH,-Ɉ/-NMN,N�K�g���^��l�T����U7��b��cQ������_�X\\�_�R���o�����Y7�3
):�Hׇ���L�+A6��z���j�ö��#U�3TqZ~Qz~	��:���F\�#�v������P�E���%�E e,����
�l<<_��}3��������<���v�y�������7N)�xUS��_���������>_ϕg?�.��3�G����Ym�Չe�G����w��-;g� :�����x� ������5����-^�Y�\���ҡy�����|�5�5x���Kk�0��֯9����k�S����J	[yc?dV�&&��W8�Zy�P�����fPd(8�W���j���BՔ���%җV�d*еe�s�9S�JR]�i�B���L+��L�"0&�c��� ���k{K�^m$" ֪������=�Yp"�Jw ���)�{6�j6���	��)�)��L��ݍ���un����yΖ��}0ei��.L�vh���#�:�W+�����8�������2�_r������f�;��ñ3�e�1ۢ!Gx8Y�ǣ{��+�������K� y�K�G���|1�f*{��8x���v��� 19;1=Ua"g=siI�D��8���Լ���Ē�����l@r�u���0��1GH���&f��y'�e'委&��pi*��\�� �F&V�Fx���A��0���z@�*u�+z�eE%��܍3M�&v�a�U�;N��v%t�%����߼�N飪�
\�=��G!L�9bHE���2��B$Cˆ,*�u�&�k��ͲVtR�)*�l�&�dl�F1&�ɪ��E�.�B���������`5�A�M����Q�������2�|�[����!��ʹ�|����z� �s�H�9��A$� �4�S[o`������V\����Yx�����ӤwD���vu�R$g!�� Hp���%a�aG'y�������n7�-�r��n���W�Z�5͵lQ2"�#VS�_���4�г������x���̰zå?[�C5�����Z�;��H�=��+_φ��}aF�=����K��������/�S29�����9�oy��&�߱��o�)`�㔳������$x�{�񈃣 19;1=Ua"��D��Ғ�Լ���Ē�����&;0Jp���h*hhy@,7�8c-�NfJ�ϙ��ts�f��,2 8�!7�	`x�{�q�c�o6�ļ��Ɍ���f@bqqy~QJPjaijq����VbiI�KI�Tm0cL�nƩ0fS�dS&i����L������R�a�N�fR�))e�1�YtaL3 �y4��Fx����n�0@��+�� bͽ��$�^��Uz��eG46ɮ��{f�F��������<{P��Z�F��u�V���@�!I��zs�iXfb7�Z���Nj����+>�˖6=j��7��)oJ�٪����
m�Sc����C��\�{�S_e	W�u�����4���"y�]A�\ޘ�,�A��a7���L��{�a���?!�b/�K��wsN���w�Ր�.�h8�O�F�e�a�,��-���X@�<�ms�.f���HEl$��"��;�QK69��'y�v��^*L�������b7!�Pd;�����Q̇���qw���{ n�H���y����N;���ךMx-�1[ ��h�u�4��DM}O�����5Bk.��?��3���{����wDs���'@U-w�'d���'x�{�񇝣 19;1=Ua"���z��	�|��%�y%�ɉ%��y�&��s�����*MNd�,�l5Y�Eu�0�'LFo�!S4���o�4�/# k�#+�x����j�0D��Wls(vH�{�'��K))� U�8"��]�hBȿW�
�P]���μ�أ��D9�lL@��0��XX�'Y��sr���P�����gӺ����,�Zp{#X�t����[�BU)���B-,��T�;���r�[�n���S�VH'<��g� IVP�^��|� 3qU��4l� Z�8RpB|�cB͇�L05�������f�Y�����Jn?�><�w}�*%���?�.�U6���1��{�2�CZ�	n���o��wّ�7����R���7�����/��%x���|��� 19;1=Ua"g_biIFj^IfrbIf~��{)��KI�^hqjQPjaijq	�����5q���!��k�'_bT E<�Vx�����y�,F�PO+(-N-��t�.)��K��ԙ<�QI"���I���pq��(�e�p�r w���Ax��S�N�0=�_1䔠(�+	q@H�=�����ʤ��"��ئZ!������-��ɓ�yo޸�ͳ�"H�vk�7Ң�7�Y�D�l�'�H�-K!�m�_\�/�#y�u]��3\BnAK�Ǫ�P�r��*	�t��v�8�"����Qi�6^���J�߬�Bp�;{K(�j'5:Oڂ#����`��6��U�w.Z�(|�>�����r��!1+�x2�� N�������r�V�+�Y�0�y��r��!6x=�$�eI ���8t��O��=�h�5^o�Rd<��$,�k�	��7,����pmWpl�U7�u�a��[� eٱ�g;��~U�3`t�l��j&��i_�uf��ϻwp�����d�0����g��`��T��@��L�ޮpܮ�V��
[C���fI��+�`^i�2?8G���DZ�=��p�<�8��? &�\�x�u��N�0���SX9%��Ƅv�� QpGD�'E�ޝ�)�M�+?�?���w�'&���Hk	���	%T��hHb�9p� ��'�Z�y돦s��6�Zk�n?,Z��O)�qbBM~m�e�x�K�O�f��i���^��F��L�l`L��^�,C]���r��jӠ��t�o0[�ԇ�R�/=�>RkrwIW�6S��K���:�l��Ϭ�.�8y\7V��t���~�����j����tx��˼��� 19;1=Ua"��yzfIFi�^r~�~Jfv�nFbQebJ�~z�nnfrQ�nIjnANbI�~Av�~QjqA~^q���fFiOϵ��3�,1'3%89� ��� �4%���x��UMs�0=ۿb�d@�w�C�Ln�$���*���L����l�	��Ҿ��v���b�W�����[�cY���(Zn)�#��t���d%i]-��E�ɍ����L�+=.�0zLX�9'L��7��GERp�Z�Y@J�a�Я@,7��R����b�8M�q5�[iɂA���@�B�K��`8���0��V�8�N�@��ش��p�KF��Z��+��� ��/�3Xj+I�{�*�w�y�[�a�%������߁�Ԡ#����h��·�i�8��h�6����9>��*FPrkߴ�����`���=ߦ�c�mک.
�j��O��l/ru��@�(3����e^����H����A���>����}$N�VgIuuYy��L|G{�����pe����s-�'���~j�c{nf��>��-���s�< �5�"��Q]�g�����	.�3�r?j9|��������]����������2����F.wGx�9<���~����]~/����v��V ���I�Q��C(L������z�����c��u:�u�B�<]��h��p�^�$��ٿ��ޘ)WJ�[�����5��m�q�����^��:>$���p^��f���P��x�31 �������ҢĒ�"�;�
��������>�owۿ6�L��RRs2�R�*8Jd��x�Zs���/����͌B������<͛�J܂6�]z���es��_>P%��7�	l���Eu��)�<��Mْ��b�0�
�3s�N�Ɵ�c�ݸ�B��������T�p	O�����ߕ~���V����O ۂT�x�340031Q�HM�)ɈO�HMΎO��K�L/-J,�/�K�g�,9�{�|��5*�o��~�=	 E�ͻzx��UMo�@={ŊCU�5R/q*;�*J�^�v=��aC7��,`b�K�,�3��Ǿ�Rz�2�9���Ez�pvc�9�C����Y�g
��eX�U���Z�sYq�
e��a����l�i�N����Vm�jud�U�B��@���O�]�����s�;�6I�����eU(��p(hUѓ���$M���� bV��2��3��o����~&33�k�o��l��%7��k(���k0"XU$�nLp�Je�(Z�1��;���nP���� ����sE�����"�vH=P��	M���:���K�*Ħ�Z���CE��zE������"P�V~F�b�4���2�� �{5=�~�xp��$ v��d���79�ߘs엡���z��̬i�&&�(:���yJ�c���jOo���l�W���ޙ�<a�H/�n<�ʢ+
�xcj?��V�a>��h$��C���Y(��O@w�&I�/�"X:n�ѕg�'������a�e�Sl!����1t��;��&e5��9]S���܇EyE�d��{E�N������ѡB�0%�k/5k
�fp�ˮx�31 ����d��f7�l�_,��u|�"�n��())`��v�7x�c���?����'~ r�٫x�340031Q�HM�)ɈO�HMΎO/*H�O��+)���I-�K�gXi�`��I��r����3��;��G�x��UMo�0=�_�専��z��SZ��Jt�4�1r����&i��_���g���Ƽy3�f�.1Y�P��l�Y?��8�K.�Kx��S��^���3\�!i�jxDx�uF�㩦z�E����z�h֋ͭ{�\*�*i�����9���AUV-C��(�k:ΰ��	�R>�)|� /VQ��(0k�/������u|�Q_% �����@R��(����\�K@�D>pΨ�R�d������:�5^��1����)�#PyyO�)Ð�m�kv���
�b�N�
y��زR�C����ՂvE��÷`���1m#@U�@�=�7-�49��� �*M�u����Լ�m��oЖm�����ts��G5�#���D8m����M�L���k���,y��@��i�6X�Z��s-9g�l�[�Hֹg 6��� f^]7@.f̝�K���!k6��a��Xj5�C�;�θ+���Z�S���
/�u��	v*KD	�pŔ1�����mG�]/��s���6����tPs7�1N�.��^�ďڐ�з�M8=�+�◷=a̿d�O�Y��h2;7X7�w�H�vՇ����f�������܍����[8���f���D7}��v}���ȥ�x�340031Q�HM�)ɈO�HMΎ�())�O��+)���I-�K�gp>��;���5n��'���~���!.�E��%mv[���LrJ]��wB�퇶)+#a/��'x���1o�0���W\�D{��)�D[	��Ε1Gb�ؑs����^��$H]�����{�n����DYQ������1]7��,�$Q�愅��T�W��ܴ�Ui��!ޏ8O����X��{=+�;ɭ���Z9;#��J
m����(�*b����1:5�r���AK�+�o��s����������ؙ��7
���_a��_S�\4吼3p?8�_�s�S����R5ʗ���bNG���#��:g/;:����`���X�i��)�ٚ$�6NĊw��%zwcH�c����5����%��&����z�L��[���!��Ǣx�340031Q�HM�)ɈO�HMΎO��M���K�gPQ�������C�Zlؓ� ��ϸ6x����N�0�����z�N[�I�A'1����2/���%�M�w��1�T`p+�?���Z�Bj�9HKy��*XJS1f���`O!kbm(߮���r�P9��U��j��XOsjDm!�he�tZ�y��⬏�ݼ�U�ᬵ)�,�� �Fh��F9�����4]%푝��!ք!��C<�jc��IBǻ��T�_Y�Hў�&i��C�ގ��Fu�rh-��کc�)w�O|2�P��4|��<����Xy���b<KRyt){�=�]#z�.�P�]�����2o��w+(y��Jc��O���������EOځ��B��懲^����)�;���3��x�340031Q�HM�)ɈO�HMΎO)��K�g���`��f��^�a������C ����x�e��
�0���)�ëvV<��Z�:7�`����PV,�K�����}����/�|TrNWF��%M
��^nW�)�0<�Ҿ�u�!�;�b�"6����P���56�z�R�l6)�TY� \o��6/I�� �BM	�x�31 ��̊�Ң�b�cvgn��l���ג?:�$n���&`%�y%��E�%��y���~�h�e��ˏr�F.Z�8 O���x�340031Q�HM�)ɈO�HMΎ��+IM/J,��ϋOˬ()-J�K�g��?��<���ҕ��M� �'��M��	x��R�k�@���n�-+��U�r馄�����^Vt���u��d�4	�i��ң��0���*���/޼�'?�L2�/�}��ޛ���\[�]/p>t	�>􂴿�Bf���4��� =ߞ {��|E�@E��Q�du����y�O�������~��=Q��[7��eI� |�˜.�W3��0R|Q�	�[���>N��&b-pw&cwTX'GΫ%A���!
��®R��.�;��$,����0�+��|���%B�:?m�^0����ow��AgS�W�����-<�B��R���P�f�U�Ð����W�����_;a�@�{�������o����*$p*����2a�~\��
�F�"sVꘊ.��ts
S��T�R��nʋ[�aEN1����V��|v'�HȔ����t������rS�a�{>�Or��p�`	�8Ԯ�_i$�rS,U����~��Ư������nfR�=H,]S�r�M~a�}��e���x�340031Q�HM�)ɈO�HMΎ/I-.�K�g_�٬�鞾��˂���MӶ  ��C��x��VMo�6=K��ա�6� =�a����"�"v���0��Xˤ�%1
���Ptb�q�E��8�����#S�l)
Ʉ�LUr�AV�2z&C����A���.��L�t���M-!-�ݵ�D!#�E�(�u��l�yfVi%�@
��J�>��?7`%d�M��b�����;մ
$*l������v¢���0��7�H�{Z�n=�����m.J�-?�a�s�T�Rص�UZ��Je�A��J�L�jQy�yFi&�~VO�Z��)�&]tpM&a�Z2�O�K�f��
W�a�]؇CV|o�9܄�����#aS	m��qB�<��Ik��{%�D>�'N�@-��OgL�� �����Zc;|'. �Gj>�m^gl�>���y�;�{(|{ 9�	���%���4m�O��o����XUwZA�b����[�q��J�z�v�l�ZBi�+	��V�Éo}4��$�0c��rlBLb�����������ˏu]�������$ql���F�ɌE jH�,c��������]��>�9�e�Q�:úr�a4�}���̓'��~oE��[W�)h��/�����ˤ/�[���ih��E�G�&6�`b拮9~�W�6%�{�pn�5?_c��������\}����q<C6
�����,�M]��Z�2ֱ��fp�����ӏV����V�0W���Ѽ�L:E����QM>8���&����h<�{����)�iu�ӳB�}K�c��P|,��*�{�"��L_A{�/*�R�n#ɽ��+eb`�zð�W��4:������xr�#4Tr7�q�O.�~;��R����+_���#���æ3�_�s7�2��m�^{���Ҡk���\��?��|i���G�_��u���x��x�340031Q�HM�)ɈO�HMΎ/-NMN,N�K�g8��:�t���q��w�m�n�x�� �Ӳ��1h�1��R�㼡{�?��f���eB��������`(`U��K������K,nCԗ�ħd�*�{�X_t��������M�:��L+�jx��UMo�0=�_1�P�w���C�JU�vϑ�� �2�D�j�{m`�%8�FZN0~�xo>@S�цC�i��C�Y��9!b�� $$�8}T�F`k9SCQ�Nd-5��E��A0�2��)�BH�FҾ�Y���)��'^s��1����W���^`DcDZ��>~�0�`c0��-��:ZwtM�-����3�Q��[�?��p"��������[XK�ޮ6���]�+%� z��4���ui���PB0�����۰�yt �Ч�%��ݢq"���z�/-��Vr�~���p'�G(��l_��Hю%���݉�R�{)�I�����O��%���\�xbԾ
���*���e[/���j�e�| HaV�z�٧%OYk��.�J�D�w��O~1����*a��Cw+���z��2�³�n�\�(V�^����s���C@�<�NDM{�����s7�y�;�Iu����Nݩx�340031Q�NL�N��HM�)ɈO�HMΎ/-NMN,N�K�g(f>i�3����c�e�#r�3?���"x�m�MK�0��ɧx�AZYӻ�˺� A=�,{�>4MJ�2�𻛾�)z!���%�T������҄��Q5�=��GΩ��r�h
�pʵE��E��0�.��7�ֵ�,�������,�Th�������������){5b���I~��"}�L_(g+�	�8�a�@�����j�
vxZ����+���.�y��p��#��L�u�.�1֨;xנ��q=�,�ʗtN+6QQ���Y��⬛Y"��mX|kW���[�&]�b���|�\���5x�:�2Ψ��wk�4������b	��5�.�ލ���q<�=� <溸�x�340031Q(�/.I/J-��HM�)ɈO�HMΎ/-NMN,N�K�g�d��~����;Wt?�v��Զ% ��B�x�}�MO�0����0=�v��}h�mNh⌲�k��Iɇ`��wҮ-�	�,����v�E�k��8_[rOĕov���ю;�]o���淺7��Z�&Ka:V�V�g^IV����S�+�I��j����M6�}RH�t���(�f�r �	���Ax��d��j�������g��&����x3�R����ےV��D3�zY�$��$W�<��/�����	�Z\o0�r�{yW��m�A�:�G����'���b�9wD��E��x�340031Q(�-�O�,��HM�)ɈO�HMΎ/-NMN,N�K�g��F�[�"�w+�?<���,�!� ��\�&x�u�MO�0��ͯ0=�u	g�Іć@���m�4���wm����W~���hy-!t�Z�[�uhV��+�%!����$�yhX���.��:��k����u5�2��,�����W�Z�&�Sa;V�V-�v�T���N	gAv��A2e�t�k6R6b�����"}k�gJ�)�		�^B�\a�MH��G�9͛���/}9�ؓ�������ES9�C����Ч*8��;��lyc>PFeF�EI�%��Z:tQ f���%Lۧ���]�#�?��k?a����U&;f�fǏ�RJ��RL��&g�p`z-��>��X{�<=�\�Q����ʏ�x�31 �������ҢĒ�"�_m��E⧼��oS���[��2+KI��,K-�dhv��g�|�egn�=�^h4�%o�E������<I{�ߩ+-����iw��=����$�a�ſϳZkv|�p�������CdS+�SJ2��
��~�2z�c�����X��'�k����$���e37��tx���e�7����!�E��ř@T2,R�R)��\77�;`Ό	�9�!�JR�K���<]{�ں�h�q������A�K�S��S%/�xb6Aw����\�'*}� ����x�340031Q�O,-ɈO��K�L/-J,�/�K�g�[-�����]Í�/���w�n>�  ������x��$xA�� 19;1=U!"����3���KK2�SRs2�R�*�Ӌ
���8�'�Ð�()) IMNat�F��NL�N�/(�O)MN-�ɻ��&f�M`4����br����g��U��M�7�|��S"_Z���X�
�4�'5��Y������Ș�y7�����H>b� �Rl�@��M[�T/(5=��$��b�a��@���b�簋�Y�dƮx�31 ����d�Ȥ���z�y��aS����6M�&`錒���w�nV���l��3����n�U��Hg'�e'2|����������c79���~�� �|*C�x�340031Q�O,-ɈO/*H�O��+)���I-�K�gx&�����?	��g�/�t4�� �M�x�340031Q�O,-Ɉ�())�O��+)���I-�K�ghXrɻ#�v�D�����'�:�S1�嗖@��ܸp�F��¹�M�9�8r3 {A(��
��(x�ke���Y��������?���5?��$c��`����B��{Q~i���>XHI���������ʔ�g��+�(�%�����酀E5�g��&��k���h�sq�rq�r �85��}x���|�u�W�z�T� ��%�������"%�"����������"� ��&'BqrFb^z�nAbqqy~Q
�g�t TEgZ~Qz~	Lg"�V7�<V��)�%���ũEy����:C�ҡPY��Z.�Z. ��Oæx�31 ��������"���;�d�Ͻ�����5��W֘��委&���θ�-�&s]Ա�_�arJ��� 	�q�x�340031QH��+.�M-�K�g���<9x׍���d%6���uB���eC1����dĜ)�!`)Φ�\� �����ex������� 19;1=U!"G&k~biI���V �
B���Ox�{����� 19;1=U!"G9k~biI���$f �Y
"�x�340031Q((�O)MN-�K�g��twgѝ�%��,J��O�~P6 �4���:x��²��� 19;1=U!"G7k~biI�ƅ��?	��x�31 ������%�u���Ů��Ĭ/\����������D!?��$#>%?713O/=��p�S�ָz��*�uV.Kby
 ���x�340031QHLNN-.�/��N��K�gpqyd���猢9�;s
~?hUZZ��_�Y�X�������
�PqD]Ѻ�n朝�6�e[ڏC5$�d�敀9����|�oC�Z�۔S����)����[�kw'����ٜ;'?�Q�k�����(5�(�8��[*'=�D�?�vا�De�9�%0��9`�Y�u��͓14���vb��?TIqr~X�寻KV���W��Q�C쭫����� �%�0�ޝ�q�??Ϸ_�_�Y  �T�A}kU��EL�V�s�j�-�����"����E��'�_������eg6 �ݗ���Hx�kcnc��PDhf�Yˍ�ύ��7Du)Uj
/��u" ����x�m����0���)�L� {�
n��n�!� �Iz�+q:ݻ��0�!q��O�=�0�Y��� >L)��1�{�L;����2�����s����AY���٧R�{������<�'�>ް��qKz���b�|�Κ���,শu�ڔ���Zlբի�5R���xN>w�e��Y�B5�X)��%��K����`gx�x�}�;O1���_a������4	��{V��k�����!QDLak��gGNƾ��h*?�7 ȧ�Y����as6����3y�p��	�F�L�)�mt�
�jY}°��� �Lx�S"	Y?�y>r�pQ�;�k�3��	�S����p����܎����բU3��^�J���$;�� -�2Z>�W��e>�L�xK�˦�h��~��o0v`4��	��Q �|i�}���3x�;�t�i�{PjZQjq�BF 9���Px�{˸�q���0����<!����%��\�\ |�	�x�U�1� Eg|
�ct-K��!@R���Uջ���l�����u�M���P�� 䙃�8��e/N�ԙb���>�Ŵ��ϧ���)be�� ������P��l�b�&��L��� _}7j���>(�x�m�A��@��ͯs�K�=,�A�=����A;)�T�e��N;��\�{yIZ[�lMȶ���<�kZEC",� <�����RY�����m�<�l����|�W�@� �m�͐��8p�W����2��S�No��Z����2'�1���0�����,a	�����EU�J���ݑ������:��w�����o���D�B����ɂoi�Qx�;´�i��BF o;�x340031QHLNN-.�/��N͋O)��K�g?j���������v��[*%V�����$#�(�*�$3?/>9?%5>�(1��USW�'5�p��GKV�P��Y����T���� ���S���{�T��[I�f�8����n�՚�WR�_\����NM���M=�H���CT�f=R�*�*/�x�Iܮ�S'��;�h�43����.?�����$|ڼ�� �l���J�pA�$������*��"��c���DY.�S"���?TCQjZQjqj�Z_z}��ĝM������� 6����3x�kf��2�H-#1/=5� ���<�(E/=�!/v{N�׬'�BB�����7�g��,'��������En���(��X��t�4֗�ދ440031Q(JM�,.I-�O)�)gK��s��:ۮ]���n;y�{�+P��)�%��ũEy��� �^tn�r��F����O�,��� ��Py��2x�h �����Vҧ��e?�Ҩl�\����� 늑jU��y�m�������dё����Jׂg�9�1J9K�lgݓ�� kUNۭU����ހ�5���Y$�3�ex�T���0=�W�*Xs��C���C��&�k��	q62FmT���bv�J����y��6/�yL�9��
CѴJ�AtlL}W%�,��9��PMV����r}�K�UjֈB�����s����댎g�jr!�i,����ÁJ�����E��Da�YƖE]�Wg�/еJv�8硹��������kz�f�����N�y�[� �T5�3��-�V�M�s��`p,�����Dr+�jp�AP!��Y�3�$��B��υBX�G��D�ȴ�܄���$�~y�M�<��`�����>�c�=���?W��G@gQ������	����#��2�Z�-�����Z[��8g�����btonž���8���C���`+�a ��%���kQ����|���Q���z㛨�e����T��l1qíLLL��E��)�67������Z�����>���	��t֧� �)�CN�I��w��dh�㠡~+�4��u��`�}B0جo�0�J�u��u�J���ʥ�3� �*������W,r�����s�Yo�����w��A̠߬��ȸ����aG>��Qo�,˘Z�:7B�x��*�f�A�dIͼ��c��/�����k+$����x,6*N�"p�)�x��R�J�@=h�Az�V�T����Cm{�b��QB2��iv7V)x�������_��;�U��ٗ��޾�������3�]
�4b�3yYM�4޻������_Sw'�yq����v���
�:�3^#w�-�wI�E���t ���A⳱@TOt�Q�ۧ3!�Ch�&Q��/�����m��3����,��Z�B���B%=(��Y��%����2e����ffz79� p这��j��r�%�r%,� a�J��j6/�r0����o��:�sm��>�;�U��p�H Wɵ��0d��Y�:mL2��ryx6�"���Ez�蘲��Ԕ'�A�F<8HE��"͸�")Kp���P��um����j�B��61W2�h��IU��~�����lQ�ui�|�Buko������$���;�H?�y�wx�UMo�0=ۿBȡp�D��[��Ek��ٵUe��j[�,um�������chŻD�||dJ!E����W΄��Kc��`�I��IT�&�v{ϥ��D?��^���85�\Kk�N�e&��u�-DSx��\�"�0?T>��$2�gl�Hk^_M�?��E�b� 5&�T�N�L�W��ܛ��+����$���{);Z�կ������p�˫�_�r�K���Yn0��.Rv��2�b�b�-�M�� Q�����zlT���ng� �6�[o5aeZn�B�Yn���-|!Y��/���K�.�ʒ*��!3|i`��nz:���y[��S���H�:�|�����ȼ���Mg��ġ}_���:�-��=�j̆@�b����'i�1�3�<���w��
h<�_����ȈD-M����E%d�
>�1X���&R��/X
��f�㚩�� >�2e-l�q��V�C���!��X�у4W%����}A��#.�?�g��;(���h�ʴ����}-�*�(j&���Θ���v�P͘z.��j]0��3v��n���J�%�V�%3J��YG=#��	��0�+t���U ���-�H~e��w7�ڎ�j-����Ap��ϲ�N����P�a�h�ޏCۨ�=8V�$���V�U�Q��<I"jf�-]��0^�o�4EBe�r٬��:�#�-��p�h�@^��	5y��
�Fx���7YpC��{j�[fjNJqXbNi�Fr~J�BqIQf^��BQjJfQjrIhQ&\,9'35���*�9�=��� &[_%]%.NNg�V
P 2$�0�
�T���`M0�7�3��  �(4j���.x���9��� 19;1=U!"Gk~biI�ƌ댛1�c ��s�.x���q�s�6�ļ�ԉϔ'�1*) AqIQf^�BBVq~��RAbqqy~Q�R�_j�dyF�����x$�Z.��Ҽd�t-�iPɠ�����M����Ԝ�bTy��
���|=�����
�J�&(Tsq����)�9+`ST���b���s��@h��[~QnXbNi��+�:@� ϼa�TE�H���&�1-����L�0�{�����e�Afu2��0[ �Ow��?x�S=o�0��_Ah���N^m$�ХI�6u���<�:I
����il'@�j���w���ƺ;�F;�zI���̤U���!�z��:�������6?�ޛgѻ�3��	���D��������DiS����[�|Ԓ���y�^!��C 3M��	�n$fb���<~�U�=o@/��D�=��e�����	Fb�z�<9ҿTU2�R�1�}{?b�׃T|�n����2B�c�(ql��1%��A����K��GOZ���|��t�,�����٩Q��k��]`��l��9��~bd!9/,�!w4��O��X�Ը��j�E���f�d�{�u;&��>$��cduv�-��r�d�Z2��ABh{���"�?�����r}�jș������ݟ«b���E���;�eUuV�ݛ�w �z6���:�Q�;Q߲p���Z���7x��ξ�sC��zFNѰĜ�T�����T�⒢̼tM��'�e��V�U�����Z)� X!P�vr ���,�^F �MӷPx�T�r�0=�_�ᐱ;����-Iˡ�z%BV���u�5����v��N�=���Io��k���0T��B۪�"����)Ũ��iי�J�vk��*���qe��1��.i�T)C�̡R�ɊiFq�S���P����׸�Xp��<� (J#���W�R�$%�����Q��1j#�=4�����nM��Q4�[��O-`k���[W��_��97z�@b�Jڸ���V���?[�ER�O�NO��O֔y�h��r6����%|So��N�ݢ��IɤW"�9{_�Pek�NKz�{5���=\r����$����|&xJ�r9������i�_
z�d5S�*��ɖ��Z�!NF�ݻ;�%��Ԋ����SC؀��G'�0ޓ)ύ�gԬ�x�I;�zF�i��D�]�f�V�� ���79�� ޫ�g�O�ӑr�����e��-�r�x�hw���(���_�8/��Z%~��o8~����CO��_AO�Z�|_6����|��v�!:���r��L;�?�k�x�V���)3�z <�$����#x�k�j����Z��Y\\���X����$����2Kt�s3KRsJ*��8�K� ���(3/���4	U�W�'V�Y%�(
k� ��*u�x�+HL�NLOU�O,-�p)���*�,HU��N�J-,M-.�*��&�(Tsq�%敄�� �2�������A�� �J	\�\ �5�Mx��Mo�0��֯|(�a�.;嚠E���uUe��*��,um����j����G/�Wd/Ճ�4��f�1�z�W�ȱ��j����M�k8њ3���,[#:�9�̢v��Q�^Z����4^8�)Y�(�ie�p"�*���~�b�#\��)�v���n�؉�j��S�j��s��9� �WA��U�Lz��&bH*�߬șb1f|�o��eG'��Hyˊ�@M9�'T�ŉkM�~wb�`%�N�G`��}�W�0�M�^�+/���P��ĩ�fh�S�Ϟ�V��)x~qF��s���%�Mڤ�c?�H������{����[n�K�3�7���A������V���j�id��z�DS�,r�ؙ7I�d��D��HVF�)D�b�D�v�ȗ��=��q;���.-�mB���W��d��iF�:p���ȣ��[9��'��#����R8?�_4��~����;$���|�3��*���	�fx���y�{�U&[����Ԝ��Ĝ�T���Ԣ���T�⒢̼t���������@qr~Lz�yFW%]%.N�P�N+�! � �~�0�(�p0���ܼ�I�	 �j2ձ�x�U�o�0~N�
S�)A��/��0Z@EbH[�`�kj���q�1����sֺ]LTں�~}w��s��KU�0�w�3q���X'�8�j7¯R�u�M��
}�'ke�Bg���:�f�n+� �M��%�p���J<!\7l���ó��J7YMi����$W�҅r��`@����d�k2�h��(M��r���e�s��ݔd}��=K��u��A�6��K2;]�(N���lA���B�^�Kh��G��ɋ��>w�W���qKr��_�w���J2}�4��qf�s���GU��<7>�ä���.�W}���/��[��x�����Q��v��ǂ�%���8:��MO	 &�o��?���$l;�s�f���:�s��@r�>��������䕆�-�T /.s�V�r����Fj|f���W�3�D鈌�b�����]��"�<���� ��$`-^�-3��"�c�Y�	W��[ .�(:b����G��P�1���!��H�L�%>��|
�i��^P9}v�p��X�M�}v�F�V�p!S���+a��|OQK�c/'D��q�WJ����$�3(u��
6�����FB${�U$r�K%aN�E��P�8��+��5X��Q9	�.yj��XΒ�Ǽ�S�?�ȓ����U�m����5�����'�G������1�����|���C�3��ϩٝ5�V�q�^y"��FW4�aΒ�D�Â�9��<A&�H)�l�^rUDG|��	�k��G|�I�Ҧ�?YqC�c܋��D�gؗo�XސPV�h�t�)-�=�S��z�������`�Z�!�}y�ˈ��,� X>�R!�>��4��hO��JR��2f����1]���jO!; �G��p+�?ur��	���y�$=�C�����x�k�/�����=��-35'�8,1�4U�(5�(�8#$?;5O���(3/]G�89� ��TК\�$69���BIWI���3I���	 �`�f+( ����� ;)����x�[�}��� 19;1=U!"Gk~biI�ƌ_��U�[� �����zx�����1�_hAJbIjhqjQ^bn��w��0�BqIQf^�BBVq~��R)TX)����+�4/YA#]AUPjaijq���{j�[fjNJ1��FrI�BjrF��s~^IjP%�	
#��8�RKJ���PUL�Ǩw���D=���ܰĜ�T�35u�8k�qjH�o~Jj�H���Biif�^h���i���%`Y=��b�����B���Ĥ�2d�:&u�]����L�8���̮�fC $��Ϯx�340031Q�O,-ɈO�HN-(����K�g��1?�Pp�zA��
R^*�+�� gf����x�{#�)�A�q�b6���X�q 5*��x�340031Q��O�K�gH�hȵ�����7k�K��	�_�cQQ�_��ZR�˽��B8����s�y[�-�<�  �";���Zx���y��� 19;1=U!"�D���Ғ���uY �D	����3x���4��� 19;1=U!?��$c#?# S��x�340031QHLNN-.�/��N��K�g�b|h��t��������z��Sy�P��%�y%�ɉ%� ��rn�Sv1ז	��K��M�#)�/ʬJ,��ϋO�OkXg˴������=�j3�6�2�C5$�d��/J-�����7Q�G�����ǥ+
�AU�%�T��c�%�إ��9O\L�8pg��|�P��y%E����% �w�t����KMn~(���]��oTa>�h�k"L$s��	l�x�_=���*�.��(5�(�8~ZS�#��n�}pz=���S��S�aj�s����-����A�����~��y"��J����j���-�:��y͵f΢����jJ�S�@J���T�/:_��|C�yɕ� QѹS��x�UQo�6~�~�M�
)Ud��^������@�`��>��H�CD"�����#iYn6�h�$�����ذ�m9(֙�oT+��a(�FiqD����M�75���SU3!!�
s��d��G���L?�R����V��M�	�%�Fv���F�*y�5�k"E ��y&a��d�n�L���E3i&E��v���1�p\S(*�������>K�k�>}s�O�����3	�5��P��h!�	�'� p��J'����[�+�8[V�d��]oǠ��d��([�W���v�jOdR��J�@l(0|3�{
hn:-�6le�S�Mm��x	X��|��-|��ln����f��;�J���/�Z�0���Lo[Z��_��V�ǧGJ�l�&v��fӄb"�I��o��-���Ґ���²�l�=��/-��$D8���W�u2�����逎�5�eL�]{	xX�<�u5j��J���j���-a~}y�)?�k%�,ў��=p/�53���W��$r��P�/n�7�/�U��C��0�����������ڐ�^���.f��E�C� ]�"��iVc.->�^�J����sf�����JiI�eٿ��;� �N����#iUU78�,���[k�&X�1�=�4ʽ/���X���;O;�t���rbH�4-����I����Ӷ%qo����%�6abWA;A(��Fes���a:0�W�S�0%���I��4���u�d�g�{,$��L�Q'v�]h���� c������|���|uu�{���s��c�#E��P��\^�K�_�N�Jx}���x��|@н�����y6
��;q�WQ�R&_��A?�E���r|��8�@�UQ_�ɱ��}u,F���A�e��4�P���C�nF�(=�ƌ��T���8}ja�{�>֯nM����y���p��kP3{ȅO�?
#���:x�;(�(����UG!��h�+���3�֛�X��q�5�&x�k�!�!���9?�$��D#��BGas$K��c�ZLh���N���x��VKo�6>[�bj,
)Ph$�S[A�ή�1�H��F��x��l*���n�C g����4~���z�`�P\�����R�����5��!~&T���H=e�7�RHe�6\o���(��|K����F��x,Źf�"����]�7hc�9�{��Ȝf#k?J���N$̆������>Si�� �r�L��xd�\�kQ�	B�A����Ȝ=��J�Q�\hH��0���.Y����߬RAϘ ��`�,�<��!޲�5��k�|�O���c�eg��ɠ�ï,��<���9= N�ab _m������w�R쬴A�
]�etMW�T2�� ���|��c�Ggp�����wo�)��h�����[*ų1����F2E�O�~vE�˅x��ae��3a_X�4��U�i�(s�����7��/̱==���c�lT♘K��kr3�%��ڒM3�}9Q]+&O�v�D��<5݁�0B��5���Q7Xe���R�F7�aj�xmEt�Ж�=����5' �Rՠ�Ff�G�ؒ��WS�?I��~w.6�����o�/��fHϪ�)zٺ̡��ܪbM��f�UT�Q�`~4�1#��[_���s@�'���Z���t���{�-�g�c��{n�»K��g���Z�����Y�|}{X��$<�%C�%����N;�$���*a���
m���(H�ygO��'�[�2�%@�ZoK�nģ��-�ٜҘ}}�h8�5�Ì*���fܥ��
;,�V��)HD{�8M�].��ֲ~�܇g����t>�ӂ����:��.K'g�ض��&���\\.�Ɛ��ڔ��4cT��Zee�D[��X������zregjkx��gf_��W{�~U �Ǎ4c�l"5�����7K��ol���$N�Zw��M������ |�0� UJ����f,d��:�)����;�-B�TZ�f����Zi�ׯ�8���LU�^(��4r��!
V�J3&[I+4���}�ap*�w�5�.��s��%���O��sI�hבw}���E,Y�,{@,��qiV��[�`��gE�{�������ĭ�K�Poғ��C��{��u��'�v��������݅߇��oSD����x�{$�-=�~�g�DO-т�t�������T%.N����T��3�hf��(��g���($�d��ăDJ�S���Ԋ�L���Ē͵LE��0C�&32�Mfe�B�l�g	� �N)|�ox��n���(�\R����W�ZQ���u&�3���4�*t6�1���
M>�v���s�e��s�M���#�8�(/17u�V�yhV�pia�z��Cs��b4%�� XaI��x��V�o�6����r�Hh��0�^�dRe���1e$Z�"�
I����;��cC���}� �x����%I�INA�J�Z
Ŵ�[�c�RH�7fD�;�h��!�S)�T�e7�Ě0Ü�Uu�bf잝��ܒ���8]�T�SM�eA4�TrR�v�Y�Zd����k,ja��lM�����4��!DRNQCH��L�3t}.*�A���\�AL7��@RT.4,��p��/z*��G�5
��F�"��$�ۅ$\��An��t�m�R�� �.�e�S�e	o�F=���`�kx��	��]�Y*%7���u�j΁=� S}��ǠRQRPZbH#���܏@#�ׂ}A9�����4��H86]��· �
�-D�R؊
V�bI��&����̕��Cơ��rW�X<���7�0S�ǋ(�����-�<0������p
+o&���~�0�O�Omz__ڹ5v��߲�%`?R�x�m%�.�[�w�4>>N/o��'o�p�ߟ���_���E/��o�x_�u�Z���'�d�D�J;$|Xp4����n9�{%���J]�����Rlb�pt����C�4�=�h&b�7�61��~��&p�^O^�!X؎��r����K������	�rU藖h�����z�!��ؠ!���LF�n�{�f���Y(:��lt��P��0����0�
�,}%������X��(%Rfۡf4���|�+���$��$ۂ�]K�b�2^V��'jt�ŝ�.�*GPeKml��-��T$��,�v�4
x����x5��9�G�U�i3]'��9��ݻ.���&�ƾ���etv���<����.�)��G�D/���'oa�\"��n�]��c��3�- ]����O�h�F�q�+�	 �^�b�EPVv�*Ǯ���|���V4����3Sk���d�؟5���#-�� �ܲÞrw#�"��qڈ��:��%G�t��o�죮X�.�.�ϱ�g�f��ux��%�N|��D���h�d������*qq*�d�*m>ƨ�8��K|��'L^o� ����N$���=(�"�{yt��!A�Ux�{'�[b�
Gr~^IjE��F�,F��
(_�B�(l�b<���j$�T ��Y�1aW�^Ɉ���
3 �&����x�k| �Y��������?�Ô1��=k~biI��.F����첓�3ZY�56g0�09sXd�7�d�a�1�YB���$�dF�ɗ�7 U1��cx�{ �"�!���9?�$��D#��BGas8S?3��s֣��B�ײo Š��{x��U�o�0~���[5MP�D�c%Z�SGW(�#u7Xq��6�ߝ��RH���p��~���G���$h��r*K])�͖1�*���aщЅ�{�_Sn��dR=�h���^qU�I���=�B��T=��%7[��$�g+%�>�rU���Da>S�<��I�$+�JJ��D�S�Y�����L�,��s*=a]ƒ�������˭�� K^n�?i��dv)��F� ���+�kf �RE��av{�]!�cJ85�1v߭�v�p�A��ȕ,,���m<�g��H��Y:{��=��hӅ,�m�:i�pއ{��lt=��J{�J/j�U�4߅O�_�L���.�Q�@nJ<��z��D����p1�s� �E������h���@���O��V��f����|<�>8�AR�
!]쫼6���}4��r���.&�0N���3��f�7O�wG���=ܜ�|�מo��%r8\1�a���d�Yd3�m<{�7��دp��5U�>=hV�'O���~&x�i��\nߞ>���&��E�߁^��}9@S��p�V7M�=�xv�5��7��[�@���yn���O��huY����>*��Ҝ��4�xd�D��+�P*�)ۃF}s����W�D�����H�g�n{,r=X�H��_�T��6�x� ���\m�����B�S����a��93�8�]��x��\Zy0	H��ĕ?�o��am)t\SZ�A52�U�ɐt_%���w#hծ�.����{-��[��h#��7*�=���/��p�Fx���o�0Ɵ�↦	*�뤼�!Z��S�M}������,CS�����i�I��0�����~����D0�s�[c�3���֐�P̂B:��S{Wb6��L#���T��vIn��P{5�$��Pii����6m-�J;$-�t8��@ژY�B�L	mk��@DB�)����y�P�e?�+ t��ZpB>lî����:�t	$������!����$��k���7��C��gG�I�1 ��~���X��z���`�]f�_a�=�7_>O�,�|�6�xx��gJ�h<��6��y��6�]��l�꟫e2�m�!| ��.VQ�ͥߍ.�'콞�����[���?T�BIFte���f~�i���G6�3��ʸ��t�U��s��M�ژ!f!q�H��EZ��>���"3��UD�5C>�O��i����	HVѲ��&����*�zN��Ar�`���	�r��Ǒ_N�C<0��L{���\x�{�q�s�
Gr~^IjE��FEIF��
(_�B�(L�d���i$�T�(l~�(�(��Mqiqj�d{F��*̚b�)����T���������ɡ�q��M�¬��颣�R�
���4�en ]�?3���Cx�;�t�i�2k~biI�D�;�d^F+ z�	 ��x��W[s�6~ƿ�,���[�������̰d���1��4�H"$���9�m6IӾ��!�tt.����a�[p�lk�c��Z��<O�7R�V;e��2�;�~���\)�4}���ٳ}�f"��B���6J亓�;q�dꉥ����k�(yn�z�b�wDf��تc�wR렳�)_�N��7��$�X�x�STO��Rc>W\/��g#i.�6K!�"�uj���h�w~;7C6�Is�jM�ǍP�����{�l���\�N��=�a�������y��8`n��!��p�a��18b�᯸�V=�X�7J���Q�B�-p�RHb^�/��y�n4��sJ�!�2�;<BCb����;����Лo�|����pp*S?Y	��P!nԳk!�Ehvf���<m=�,�Ρ���l�p�,t(𧽻�z�Z`�M�<�̅����WU?�ka�~�r��[ړx�� �\�7��
����aeq����E�k����q�w�$z���y-�W�ܻ.dbgg��M6��i'��m�	�}�?�\��E>��'l�g�+�_�L`4�s@��^���i������)���r��
$0+��M��(��*�dR[sli�!�4�;2Ԗ]��S߲M���u���[)Wp����&��]�%O���}�-�D^j�b4BD�h6����9��ߜ[��� ����KV� � �(���a�� �K:7�����$B�I� m+9�����S}��2������i�}K��ћ��j�M�����-:���n�*�Ve�`��ڳײ�[���,ps��
���p��u����PI"~�fxM��+
�78��d�m�oT��I<�����G�-Bx�z1��������3�~�ׯ�v�x:���7W��x�v��� 8��ɡ�á���DoC����ha�&7pBl�Qj�Ih���@��X"4���g��Ɓ������+[���� ��<�Һ|�9ꂿ9�k!|w��3Mc���N��͕\������2�� 5�~Ш���z�/�M���^<�c�>r�T?�����%�&�G��,������d��y�Z�q���f�W��f�
�Ox��/}Oj��D���D����R���S��8�J2sS�6�0M`��K-�QH-*R��U�|��n�=��dY.���د�&pqr���e���i������N���|�#�Q��(���(OY
lf^f�f�'L�0��&��nF��  �:�	�x��'uXz�
Gr~^IjE��FŃ�"%
P��3��Q����'#/���\R��lĄ&�Ǧ�.d��Q4���\��f�r�d �3����Yx�[���E� 19;1=U!?��$#(� �8�$����+3� ��DAc�v+Xrb�����5&�`\ �>��Gx�{�2�u�
Gr~^IjE��D����%
P��3��Q������� � �a\ �f���x������� 19;1=U!#�=fF�͟��1+k��U�(��%�)�����z�ɉyj���y%�YD'e7  zZ��Rx�{����sf^����Ԋ��
��/��1K���U�('��O�� �#��	��Vx�ۧ<U~��D�Ic�y
����R���S'7���|��&��9���;��Wl�K^$�@F�͜����\�Z��GM�:���
�$7�	80 �6��dx��*ߠ8A�#9?�$��Di�"#��ٌ���%
PA=g��0���d[���'��x��@�@n��ǙgM�Ƣ�C�^��&���P5n�ˈ&��әQ
�!�ũE
���l�����m2�MB���Z���fV��o=y;��R@bqqy~Q�vcJ3S&ss�s@�MV�
��_�Z2�'���&>��<�������C��琙�+ ��x�31 ��̊�Ң�b�/�D�M�7^��n�Y�ե<�a�	XIf^IjzQbIf~^1�hCaa�������K�f���� )M��x�340031Q�O,-Ɉ��+IM/J,��ϋOˬ()-J�K�g�zV[�������y�UU�  Ot���x��'�Rb�zF���Ғ��{��,)�(��)�z�����\ˤ�������V��_����ZT�`e��霟W����Z��Z��<�$����26 /R&
�
cx�{)qQr��������̼�"+[�LgO�ά����L�Q��K+Jt�/��QH-+OFU�ZQ��\R�������R��	�	�/.I/J-s܋
����| C��;1-;��K-�t�ؼ�u5 ��4��x�340031QH.JM,I��O,-Ɉ/I-.�K�g��i��˶{e\$�j��� ��Ʃ0x340031QH�H�KO�/H,..�/J�K�g���J��L�/"&�t���8��2BU�d��ė�&'��q][��K����S&��v�U��_��_�bt��i{���+���դ+�­t	TuzQ"�������������<���B�l��9k�X��9������]%���%�E�U�%��y���)`�y�Ҋ��iV��pH��ܒ��ɘz�!^J.JMz-31�d��2�sǪg�Y�~�j&��s��ԋj�7���@�;zS*���#�T�����Q��V�Z�����y��M/L����������;��j��+)�/.HM.��p�6�^�x�|��{����n���Lph�[����$#�#��0oo�p���TM>(��#�>S��u��ܽ��:wg^C�{�ZG�	�8UƔ��g��a�~�c��D������"�O�ONnh84W�)/J�����&����Tp�z����k�Ց�Iߊ7Ϭ�/iUS���K��e}��s���U�ٞ��>�`j��� [��X���J<<���O�2��sAU�**F61��������Rښ��y{s�S ��8���Hx�t ������V�u�z� {�b CU�ʑ��8����V�l@{��TtU�@'100644 login.go ���jˈ\ţ��nJ��p�œ�k�U�W��C�O�_�V�g���g��m4����x�{�q�c�6�ļ�ԉ�t���:�20�c2���q�̔49���:��dff�����z0��N��.j΢ �'����}x�{��}���z��	~k���R���S�&'2^����&2E#x�{��0 �z�
�6x�;(x�Y� 19;1=U!?��$#�8�9�8��+3� ��DA��S)9?�$�b�a��&WZi^��Fi��V)D�ď�-��S��C�S�tR���lJ���R�3K�*�܋�J�4�K*t&��*L��(?�?;�TQjIiQ��Yy�9\�\ ?�:���x�;ͼ�m��Dsm�������᜜���%.%��lXK�8�����kg����9&'���g������NVc���LĔ�QH-*R��U���Z��.Nd�:@n^f��8L�)�8��t,(Ѓ0�A��A�$S}2�RK2s����+�Vd���A�)5�(�h�&gQjIiQ�Vgm�� <`b��1�+x�mR͊Af��N� ��"H���;����B2Q�U�O��[�i�t==���� "�o�A|����|�3Y6��K�Ow��}]�����<�d����f/
����\m��V�m��t����Հ��Ͷ�Ͼv�|<��/��,r�
�VN�F����.t�
"�>�i#�$Vj�=|he�����Ǖ킻S�w�j����z����MoY���u�>�{n�m�7�}J�源V�� ��N���ѻ &�
��~���!�P�A��S�Ux�����gnpǚz)ynm�}l��'679��H6C 醅ɑ�.L�0[DG|�J*����m�bO����?K2��䚍�7���0�	Zt���.}�㊫'$�i��Yn$�zr��?F�Op��mo0�8�P���¡�M+s�j�5�����S�G+��%.�����ho����9��Ǿ=�Υ�;2E�}����01h("Q��A[��Vr�ޱ�"o��nx�k�m�� �ʚ\R���Y�5� 0�ʵyx�T�n�0<�_����^ ��i��$i���	K�JRM�����C���@�^,��>fvf.��E�|t�oW�"c��q�`Y.�r��r��J��K����[y��f�kY���������q�5�����F�\*8!^RM�xW���	�^�ؽ%u�2:}�a�� �*�v��bΖ�5������%�:�ʭ�������+����*�_�z@��H�`���'�E�v(B:8��^�KX�S��+!����, ��f	�XVU��؀��T�/�2:z�Ua��FQ��N�]y��;�d=�t1︀��e������|�̠��!=�~��e-"X��P�E7�����L>h~��<���n������=�(|9,�EV(P*�Q�;�ǻ^�x�ݐ�?�8YvR�$)�!M/��߃��6�\Hk|9���.�~ L�I󝔒�P^�>!�6(���˙���eS��Y�;٠�=z�d���	$�;�$�Y�{��(FZ:�ÿi����*�����:�09�\8�wo����=����B���>vd �� h#I]��v��Ł5<��=^�F�ZQ^��W$D�{)C}mf��;����,I�	D���ӈ!X�Lrq�G�k�V4�a���0��<O�V´&�ӂ\]���O�@tc���.](�P�y�=\�v��~laϮ��N2�h>�8��3�N�+?��]@�⊺�Ɣ3bH�M��F� KG����bx���?�C&krI����L&V 8|X�Px��Tˎ�0<[_��d�|/�C6mݢ�Pe�bK%���{)ɛG,���rș�<(�W-�Sc�}��Q�0��(@)��v6�C����xjM؍?�v}ݘ�Y�=��ԭ[�F�[�N��|kZ�I޹^��7ܝ���T_7	��]��4��A�u��=ө��S�kxO��ީ�4, C�xK������R��Ōda�'�/gS�S"8�aJ�U��v��Q�|�~U��uMʆR��l���^�4���m���Π0?1Lnҷ
����r�5z����~A?8�:��g������"E)ޮ`ԒӾ�oq�i�Jf�Rެ��.�a$�T-�'�ڬy �h6'��LE��m�y���S����K�Q�d�#*�$0͡�E���LzV�/�o��d����k�o�qvu�6����	q��Y�lY��_$�09�&�837�>l ��%�+q6j{��O�W���|\���ۈ.����l16����+T�\E��<�}�D�3œ���,��x���:�9Ag�	��	�'�1�s�����*m^ϸ�����ϼ� ����6x�;�y�s�J&��
��+��� C5.�Ix�TMo�0=�_1�	"��+�l�\�V�~�]3��`[��nT�wlh�F9t����y����G�#85��-�^E�L��RW�Y�]�goh�H�5G�TxV�iz���nG8�Q6ԙ�cr�{7)c�x�5�Uc��6'h&���7)���H�2F��t*!��j(g��"V���;�}�R���nr��W���v!R0��A�-��F'��{��.9y�5Ƙ+0zg#ր!�P�/Q4< �h@���(xk0������]4�³|D��F�n��+�c%
ӥ����S�" ���e�+���p��%�E~���¶ԗ$J��ez�7_T����~Kbp��(ԫ|�}��[Hel���ޠX{R�s�3���QA��yaLKSϠ�u:�l1	���|�<�gLy�za���e.�9��GT�z�,ʟ��@.�M�'�!����.Ó7��3�<�;T!��wʫa6ye�./�7��H��]x���9�s�krI���f�R& 2���x�V�n1=�_�r�؈�;IK*�RB�c�,v�+�7E����^X A�T�*���3~���3��1�Ei��\��H��6���FS����6�q��|�R���Z��0+1Q��>O�4��B�'�B��j�1.�P�Be�x�g�L$ߙ��TO Y'����VWQ��r�(���H�(�@�c0ЙV��GѴ�$k����^���2kt���c���%����W��,�
_�U_�^VX��Y��D�<�)�/�Z�Za�j�9�6@k30F��=G5�����z�٤7w�f��9Nx_J(
� E��(j�D��p�Y��r�����`g�*CA�
���e�Vz��9���ߩ�����H6v0��c�J4\/M�>��_-�/�z�.�����6�z�T2
�o�S+��)6lˣ��\��}�Ǹ��8��IY9ߡɕX���v�|09�H"�)���^�?��5>�;d-^)��\�j��}�MIT��4�#
C(w�9M ���~�^�ֺϣ٪���#�C�-LQع���������$���X�֍Jh��\M��"|�������2q�!q�ڬ�D~i�Z�Q�H�@#���A���]����Xk"��ڀ�]��ct�㕽��t&�ʜc֔�ƥ;�s�+�xhܭ�+��,������X�U�aO��}���8����(aҷxBmk��k;��#cd4�d�W��h�L�����0�+@�A�U\�~�p�Kh7XV.Њ�p�!�^��7X�"ǿJf7^��j
�|�w���_:4e�D
��G^�~�Ļ��UG�붎���0p������۝��!�{@5D������c��;�c�>fg�ݳ������^x��#�Zz�V��
��OX2��Y<9����� L�V�Jx��RɎ�0=[_�ɥ�����f�"� )��T���ؒA�YP̿��\����[/Z�<�=r��,+G�~w�����`�C*������o��hߎ?
e���g�m%�e���n{��n=�C'=��T�s&�g�Km��5�$#�r�/�	��m��FdB�%�؆1!�;��R�x{F��
kB�Ζ�H��m��/[�i��z4
�Q��%�"~��f�}�sP�F��qA��O�<XyXTr98e�I�&�t��Z����ɧ�av!��~��8��D�z����A;0փ�:{Ŋ^�-�9���SFU܉3v12 E0���s�\.���e�������Yô(v!�A�$������Zq�`t���#��\d�����=�t�j�ɫ���2���j�"�K����A8X���{��My��8��_�b��x����C��0�"��]��=��-�m�<���˨٤k���)Н��C��eʚqi�֫�	�݂�pF���ܙ�����YNs͓�ū�������x�[���c��Z:<���E���yũ7�1N��(Ǧ痘��99�Y�&�7������Bg�晛똕e�WoKx�{��c�J�ɫ��Nvb�  8�"���x���v�mB;k~biI�D��<!����P��|(�c��咟����睘��P��R��Z�å� QV�� �2 ��Ax���An�0E��)&ZI�M����)�n\ I@S#��D�#��Q���j�&R@����?3)}V=�S���ϸW3
a�ɑ�J�v��/��D�f>%���P�Ɵ�Qj76�9��I�U����v4����8�cc؉��Tߴɠ]�C����ܗ�⧢�c����p��>j71����{B� �R��C�2�<�*_�0G1褆#B��e�����?`G8���m�B��Kq�X������VC4܆�~�O�J�gXf!����c�dp��\DݚfO��5T����5��i����u�����b����	BG�7<����(s;^�vp�4K.���>�|�(8�8¯���plj�v�-�.	b=��43��to1ovP�*�$LF
ZNn6��Urد���R��e�e͐̊<X�N���E� ޮ������	�Ze�ˏ�H�K7���qR�p0���$ƃ�ˢV���!�;�h�s�K�bˤ�E���h���mx���q�}��D�VFΉ�ڼ���E���yũJ��1q¸z�2[ �*����9x���|��� 19;1=U!"Gk~biI��{������'72�L�� f�N�c��8e<���\x����]� 19;1=U!?��$#�8�9�8��+3� ��DA)��(��Xi��f������O�- 5�"�Dx���>�c����RjQQ~Q��U\R���djnT��,
����䗧i��jN>�l V*D���x��˼�y��yzfIFi�^r~�~Jfv�nFbQebJ�~z�nnfrQ�nIjnANbI�~Av�~QjqA~^q���fFiOϵ��3�,1'3%89� ��� ~<"±�x��WKo�F>��b�CJ2e�7.`8M���iA`�ȡ�5���.���{g�HYN\�H}���y}��̎�߳�d�Y�x�4�1��$q4A�˂��l%��])�����f�.�\ֳ����5S;Vp<�y�������Y.E�W�㼼�5���>�`����
g`V���@VR�b3�Zs)\��lmL3��8�0ea�f�0R򅗼f5B�eqt�����w���;�j��.x�8[�8~ǝ�spv�FK��ߔ�%�,So���9x�k�&���4���$�PͮAX�'�#ݨ67�w��-L�.S�;�ocO᤿'����D�$��~Nl���W����v|.uC&��Du��e+r���$�QS�>��k��P0��R*��H#�PQ�f�RB*ɠ��=ij��g�/��U����N���}t{�$%��2�.ݟ)�=Ns
'�I��}��pD'�>C�s)�9v�nhw>�2kʎ�R���U�3z��z�8!8�E�(76m������+��b���{)������n�M�w��ﭔUy�_��� ��4fm�r�L�yXI@i�]�j���KtOC��|�ƅ;��Zuթ3E]��-}�v!�v��p��T�'��5�P�Y��0,rVU��K,m�H�_YkM��x�5Ȓ��r�l���}J���#��?17ǀ��^s�e0'���砳a�eo�${zd��q�K����������R��(]��^�TB�ɠO��� ���5�k����z�ݔ�b�T.-%~�ޚ�D��p��N�:=6�-�0͌�!~[5�J�l��)�tk��8j�a��-���:�<~w�dO�4� 3�q�����ܼ��دN��)��gr1fR�>�Ǳ^2�O�����編�C[��Dߣ�J����^mAY�MFs�{�.+dj�Kڂ�</������k��X#�"9���`�J��&"�M������f�&#�Ҟ�u�S+Ha�m嶈�kZ8w��R��̰�>#?����SpE3��B�Kj���L�~x�/��%x�uX�$�b�A>;�o:�%W�t���ɣ�Pv�T�h�y���0� P`�T��ۃ����+'@�6�o�a���$Ȥ�+�AX�>!�zӍt�o��L����[)~rK��Znl�ѓ<��C�Ͼ��a��c�����Ü�G�e%��?j ��
a[$�Rz�t��L��]�� ����3x�{(�[f���n�<�����y�%�y%��������P���o3��p��mvb�f���M�e/E����*)3مS�`r.�s4�.#q�|hqjQpjqqf~�wj�dF��7����|�[͒͌�	� K��
��Mx��,�-(W���������XZ�Z��X��ŕ�[�_T��1QÉ,3���� F�ɳ3'+0g ���L�Hl���&�琜,���RZ�Z4y.���/��d'V�ɊLV��:nVe�a���>(i<Y�#�l ;V�SGD�%�jr�r w9��x��|-4A�S)9?�$��Di��hrI�T@�B�(L�bta� Yy��8��3IM���Ug��J�:��~�8��1GAս`�&�f-��05��2�L6eu���\�*��0�m&NC%'�rHM�cχ�=��W��8X]iqj��K�M��� ryU�x��1�  ����I�����V)��ǻk\��
҆��� �%Ws][�<R�e�b�0JH٫P��XˎƁ4v K;�+������8�L͸��$"G�x340031QH��+.I�+�K�gxo-�%��ӅN+�i��ϼ��ab pU�5J����j�MV�|�p��I���Ԣ��"��l�v�d�Q���jw�����"W!6$����l:��P�&i��Z��!�;�kB�)J-. :)��&��i7�Y�,�.Z�i����BM*.���m�D���k��	�Y�F��Ǳu0%E�y� E��⁥�+�܉w�k�N��?��UTZ��SRs������6�:��Ͷ�>9�	# �r���x� ���������Y#ݯ��V�0�~y�(���k�y�x�+HL�NLOU(�N��J��+.QpJM,J-R�UP���� �j��x�340075UH��+.I�+�-I�-�K�g�3ɲ�J�T��Tht�h����f&&p� eWָNu\�yzӉ�a�.���:���(��1l^�4X���c��J���pc�>1FC��%%�%�9 c�O���<[�]�gGSX�����҅�����ZĐb�y�/�7b�t�x��D�] i�Mųbx��SAn�0<��X R$r�����p�=��Zf-K*I%���KQ��X.��=���,+.v<Ce�/c��B���I��J���Q���w�c�S�V<�.k%P?�3���b>H����C��q����U{B��W�НPMe`��f���`9���
������b��'��K2��[>#K����7h?�=�ѿsҭJm2��uD� mU�x��9���U�0�m,jO�A�Z���&�^�܄�q� ��>�rް����*�����0};B�&���i|Aw�[oGժ7Ǌ�:Il(�HI����J�nZ�/������q�;l\+ۦ���V��i���9B�-�l������a���F���[u�Y��Ԫ� �D�קk
�Q��Dr�9�EaHG[��_(=���yF���Rf�"�{)�_�DW�*�؂2ܙ	N[m�rP̂��lO謫��=.��$��U� ���z_�VbJ��R��1���s�n���5�Qdr�Εo&�{Lgd#�)g�:�%��>�E�.
��g�i�>��dMW1H��?�H�c�x�]��j� ���)�SKY6K�cK�d6%м��iVjQ�ҧ��٤t.3|������/>h|�&��ҽ���<��	hCY��N	�\T6LV� �̻w�J�{�2;l��_!�%ą�l����/oC��Ί57�#��Trh\_0ۊأ[ħ��Iul�=C��3!U-1�2/	}���%}G~=�;�����[5t7�԰H+>�O�t�f��u���0�<�А?�L�t'��ox���|�e�$�z�����.N���������<�̼[�D�X|	HP��3(5�(�8���"S�Ʌ01�$�(1=5hDf~�_bn*HCz~|~biI�Q|qj�mYj�+ Z
�*�N�T t �@d �hBЯx�31 �Ԣ���������<]���Ou}u�z�)��0�듡����	TUIfIN�^z>C�s��wM"�N6���~u���_�� ��!��x�340031QH-*�/���,.�K�g0��Z���� ����w�SN�  3���9x�uR�O�0?�����[�$&�$�u�q�������1���aK������4��L)���Є쨂g��4�We��~���\ԡЪ�5����f<��K�P�O���w�vL!�~�z!K��چ���z;��Yn
c��9{l��؀=r�!�L&�&'E��L6�"1�Y�KK�o�,�r�Ops1,C�}��0C�`*y��*�ڵ��RFH������S`'�c�̇���c���\%��*�G8D8Dx1YO�� -(�F�v�@������Z�n�h���5�6㊩b?%3�۝ؐR3��3�ƍ��b+��Ԓ<�Qm�-W�-�s;_?���DN��c��1�&��`��o��#x�m��R�0Ek��T�<63l��� ^+h
���oXW�9��5`����*z#i�1��?8o�6���k�}~�	�w	�1�`[d+��_�E�l���J�dL�o
t�9�?D���uSk�v'��!PZW6{a�.��H�zrS'��]ʤ	�#�7i��S�8�w���y��3��@��\��3�*h���n��q�O���\�/9�Br��k��ms���>�d�mx���]o�0��ǯ��j�j�n�vA�ӡt���i�"䂗�&6�M>V���L��.��>�9�=T(AKrJ�@DXV��(��zg,>���l�ڲ��$j�����n� ��s~&P���$�i1d��)e�q�*"���\��R�B莌���;t�&��Z7��&-�1�b��;%�+��߸9Ҕ��bX��p��:�v��yՙ�ġ%�P�ʛ1n�%�#[�\&�^r����UJR�#QRr�ydtP5���wSZ����%*s[� o�~A��mP5/�X�\��,_as~���M�8g��;��vs�;��Νd���8Q��tP � k�8�u�tFh�n���-��n�X�Z�N�n�(Yȓ�:��C��	F��0�W���KM7M>���NS���v�e �kR���wY��Aم�aNk�c�B<cļ58tg�z|��m �5�zf^(�$��^ca���pFw�d�;��D�9���Es�T?xr�I�f%��{n���I�a�ȟ��`������ZH�F�8��ֻ8�� LdmiЇ�Q+��e��I�@�_����^�vy��^�E��q��J���.���� >*��X)ܥx�340031Q��OOO-�K�g(/xv}�/��㇘fw&p��_�� ���x�U�An�0E��)F^���²G`aHjOTe�"����
��:c'�d5������t�3�ަ�|����9^�J�d��lbg��LZ5[���X��q�QD��R�i�:��{0{xyA���2k;#'�{L��g�A�uQF����fK>`e)�Nd��H�X]!��u�&����e���6#K���!-rF0ƺ�(����3�p��~ F����5�,�!<�a�3j�����jx�
x�31 ��������b�E�??ͱy�MnÂ�*M�{7l6��)-.�ύO-*�/b��/��J0i�{�D��n2�'�C����KK2s��[M�x#S ���w���>��	�*�� �!��m߆w�����;�[�r*D:��������5	��/T��'$�*�|� %J��x�340031QH��+)JL.)�K�g��V�m��
'�<`u�y�t 0L�x�E���0k<ŋ�40#�!zB� ��)bw"(�,�|ɺ�z&���]�:�(5�jt9��ŭK��^0�0���i�bˋ�®,s�t����|���n<]a*f�#x�340031QH,(��LN,��ϋO-*�/�K�gH����2c�Sk�w;e&�z��_��>)1%�(��4���^G��v;ӏjn
�n��� �U����� I��Yˡ5�_�S��j�Ҟ�������"��TIީ�1�Er}�s��v�B����&f"9����܋B�K�&��[2w�w�M!P�i�EI�)�H��՜�X%q�謃�C�Y34��:Ag敤�%�����!�X3��J�b�?�/��?�;�ۡzr��3s2���M'{�m�\ßt��?��+��P�y�%�i��y)��k"��L2�V�tk>1kuTui^biIF~QfU*���;�eϰ�
�Zr��ȴ��ք�5`s��ً]<����E��xݪzQ�u�2TCPi
Z
�?[���!�,~�����1A��s ���}�5x���1O�0�g�W<u@6�ҽ : �yI�&q�B�w�	������^Ƚ��`�n�цR���8`��j�vC�Kݮ�}�� �+�)u=B!�G|к�(Xg����ۙ����I`���{{s��8Z7�ĠL���!2Q���JHLS�_��R���j$xfQ2���*����2��b��[�m���LJ<⨯Dcqb~��Tޢ���᫮�@�ñdP���Њ�9����as� ���܄��?�Ts28�Oa<;�?G�5�g$�LV�Y�O�/B��Oċ����Q<�e����(�Bi(���<x�����m�=wbAANfrbIf~�DOy$�kQQ~���g�#��>nd�%]G4�
I��9\�\\i�yɨ�'s2Jˠۡ����Q���\��E�%�Ey
�z��3�"�p�>F/d?N�ͤ:ٜ�EMS$��j ��jm��Fx��ƶ�m�={r~^ZNf�DM	(�ĵ�(�HS���*׫�c�VVsFV��������U�ŕV����4��QF�D��-��!�]�\��E�%�Ey
(2z���`�������ɂL�p�r�($q�I�a �PڹLx����n�0�g�)�Uhّ2Di��Tb(
`d�VU�w����I�T�,Ⱦ�����ˏ���J��QJ!)��NHQY��~��Yw�r͍UE��.� �e�k8QR��˖�;q��J���`?t�ڒ���gu)9pͪZ���u�����|SZ�m����$y������( �Ir������\��5���"�9��Q��.���G��x�-�~���n.�?�Ρ�E�B=���'��(���h��-�݀�'X҂���N���%�^Q2VO�B)g�P�L�;����q=��v3��[eB�={�������S�j5�^K`�*VЕ����U�A�[�m&�B�i�,ȗ�1�ཙK���d���)���s��ߦ1��סL���a~ ������Zx���6�m�=[J~nbf�DOQ˵�(�HS���"ѧ�e�QRvA(��TH������J+�K�k���(-�d�T؎Q�ņj.N΢ԒҢ<$q=�9PK�8k'�b���a�>F/��'�fR�l�d��`�D�T ��M'��dx���v�m�=gZ~QRfJJj�DOi8ǵ�(�HS���.ݧ��Q�tCQ��������U�ŕV����s2'����
I;FIStk��89�RKJ��P�����gTE�h�>F/�_&�fR�l�d�$_��"_ �\���kx�����>��/3�$�(/1'8��,�h���*��kQQ~���g�'��>>4�=Jƞ�Z54���s�j���J�1���(-��CM����M�j.N΢ԒҢ<=��U��9y���'�fR�l�d����)Se5 �3u�7x���?O�0�g�S�: E�^�!X�\��ZM��@�ݱ�&�&t��|�����NȽ�dp޴��K�j;c=0J��)�i�ݾ^bjp�)��BЏº�h��bp��ዒ���7�U���D�������1M2����1O�Ai��Og������t=ǻ�3�'BJ���֕��aW3��a�lrZ�"Sq{M%��~L-:�vWkH��tMl��8hE��O��g�����Oأ�]�ѣ�U�'p�?��ş���Vt,�d���a��|�||@�9�W��O�9��ߴ}c��h5���>x���v�m�=c�D'C��Ģ�Ĝ̼tע��"M��]zw�L|�8��QU M��dFe���Y��BhRz�œg3�O>�T��Ior ���.�d0� Q*���$x��¶�m�=G^~�[~i^�DoI۵�(�HS���&9E��>�ᇬPCS!)??����+�4/ID:U!bH�B5gYb��U
0��%��\��Y�ZRZ���Ia�䥌�p�M�����d����Hd��r�a \!^L��x�����m�=wi^biIF~QfU�D7M$n�kQQ~���gqh�#\M�*w(�����Ȳw%P�dd1T@�>Y�Q����\��E�%�Ey��Cq��
F5!$>D���B&d�M�ä6Y���u�������  {�ja�>x��S�N�0��8u��{%��X@�bp�K�����!�ǮIJ���ܽ{�޳�	�������mSM��ΒY��K�ɥnݶZ�Ќ����a�*�U�ݏY�K,��=��H6��+�VG���R����Oa�u��޴��ѓ��FhP�ES
��6'?�O�N��*aN�FC�%�s���c���jt��32Y�$��RԄ��;|����t���*���dP��&hD�����^��ҳF񍬁n�r҆�Dl��(�0�W<(�>\�%���/�4s�]v����ڈ�;/�O�Ÿd���PȺ��~�ם[�����|���ۈt�}�\M�x�340031QH-*�/�/-��)�K�gX~�������t�%��Tc�v	>X  CӰ�x��Umk�0�l��k`�f��#F�ؠci7XW��Ȏ�Iq۔���I�㴁�A��{y��<wj˲��RR�Q�0u+��(&�l4|�k��F��Г0:V�53R��f�]���g����N��g�C�l(wՖ�����!�튫T�b�c-�[���|-J1�0��ւ*�"Srjx�V�p��-k�����}�@��3)��3���|6��m�������ۏa쌔��U�(8�a8��bó���Z��22iȱ��L�"�zpb�L4Df�%�m������"�B��N�V�<���B�|�5�2�`L r�c�.|Qo�V�^�,����\Q��)��8��'��m����³�-��}Hޖ�J�poy�&�Y܃kԕ��РЌE��:P`����Qã��w�r"ͅ�x3&��ށ踇~��Rsi�7;��`�$ K�(���:= ��Q����o���ET�G�7�_̈́%#�!%a�Y�g%����O0w����yP��v|a�C#*�ߧ[%)��H����ΰ�bͺr���լ�u�ܹGһ<w�f�pI�Yҡ�:�`�<��C���5GiQBd�f%]�Hj�	t��?���(��r������1	�d�%�aՒ��+w��{��6L?���W�Q�@.�&0���/��D�/\'�W�8�X�5���$�yj9x�>���f�ѷ^������5�iC3-
��G��쵁��q���p�k�FxZX�5��a�]WN眫��f��x&�Q���F3X�����[�Ѧ�N�6]n�Hn h�:\6�䛲���E����5��nl��a�\��7���;�6�+-|�D<�p��`�E�x�340031QH.-.�ύO/*H�O-*�/�K�g��4�u�ܤ#\[�U-�'�������U���]��I��0=��DL�$��
���S��/nTN��^���t�����l`�f	 +�7���x�͖=o�0�g�W
��=��A�E�t):0�>�_%OI�"���h'�E�@k�p����#� ǻ���wݕ��Wjg=�E5�j�W�AZ+��*ndm�lRB�Y!m��ziM n��%Ҿ��;��\��i�;o��)Nи��Ur^3T�W˪z�M�v�tU�q��Z3\$Q��VLC�| �F�� ���}ϱ��d׹?���y�����r�̾")�`�z�I��䷱8�a�*&]F9�%K)�)�c��-E7�VZ9:L�[���!=�7�1I�^�b{��X��v,��zg�˸�R��g?(����ddZ偋�_(���/𳏛��y�?�S�����E؜�'#s��δ=���P˝�&C�E�q�.�q���ߣPf��O��3x�!�y��#��o������X�p�d�Q�6Vs,sz��Y���9���R�9���qi��A������x��V�k�0~��
��a�Tft���5[�VF7���k+���2��.�����r�����>���t'�I�NrFrQ�_��"���B�0�LUQ�iLz.���zlh��8+���c"�IVĵ��?4�s��z�(Kp�,��������un\R;8�7��|�T9�"�1�8�;�*QDA��u[�J4�"/�䇎 ����������O�"3^���N�IVT�����z>�g�6�/�|�0�G}�.p/������TRl��2���ͭ�fY2(��	�K�u7K�!_-�זd`��U�2���)Cu���[�����J��¢�_1-~��BM�:ژ4���C�c�`u4ؒ�r5��3+�N�0]K&,�2n�v�w�^wd/ux���,Z_P�c�-�5�$w��VM���|�z{W�q$�H����$�~�\�������mù�����ߟ����	LZHc3�6��F���jU@����-�sC�#p �T#*7#Z~B���#2 Kr8����CP�}j�a|�	س�)��q됅�>8^�א~R����4޷6]�B'zZ~��yh��D������N˧?GN�M�����!�p���\�)?5�?=�k�vAH� N�rd%�!�d=23[q�)��A�9~��d�v7-��7��S"�E;��*��B?I��Y��qLv�2I�K�$	�o�/�t6�(N$�2�9ª���{�=�+N]���~�v�ɵc*
& p�27��Վ.�x��^�Q�e0���߿m_G�Qm6�`Rw�g0����+.�Di�������⃙�P�/�A�%ȝ)���a[\�j塨f���D���L�l������U�K^=1�0>����Q�����R�X8g�CL0���>bB&�e�ԫi�i{���Q!�V�?�153~�=����T���n+xz+'�'���s<������k�Yc�5��A��x�͖Qo�0ǟç����+�!KS��RMk�=V�����}�ݦ~����nl�K��ߝm��\�x,A�W�h�y2˵A6�Fc�#c�nb���X�\Ł6q�\�BG`K��K�,r�lKL��@�,��N�n~�H���eR=C��#��.�,U\貌�l���L��mJ�(�(,����ڴU�*�m�{޶P�}�Ƃ�:��F}vY���j����,첹+�}o$�����Lɴ�1�
�(�ހ��*��=7*����'�JT2���nGp�ꯆ�%֔u��m�=��=1-���V�/�>2��Q�h.�v�܁�r,l�I���S����HL�\M��r�ډOh��[��y�E��E��
na��+���2�(�j�<+;�_�츠����{���=�>��,��ʆ��x��b싆����P���]hs'���Ɂjx��jQ`���	�t,�\g\�7�-`��/��#��(�*����w_�/vǚ|.�w�����yS0�N��^澯���b��j�0dj����[	�I�#�$�@�㧐�*�Ð�6�|֖�Z[�Ff�����K~-v�8���H�?����9|=ٻ���s���jx��+6AT� 19;1=U!�� ٵ�(���+3� ��DAc����y{Y܁2���%�aj4��SR��<�JR��s&�q�O�f.+����
a�4s�~�%n��y��"'eQ�H>�H��ƪ�am�0�ؚ!��lM� �cAANfrbIf~�������+@�foE��C�i����U�\m0��ٹ� _W�x�340031QH.-.�ύ�())�O-*�/�K�gx��Ǜgk���"x�u����!�Q>��S^l�g�P���u}'C����Eũ`����FZN�Q�3���,���� w�7E��x�͖;o�0�g�+�H@�n�ڡ}-U߆+�G��Vm��^;!@%��ɒ�\_��O�+��ȁo��s�1��G|��JC%�τ
�F͍.Ih�I�����̨T�'ᾄ�47��3eA��m�f����%a#��*��|�x�gQ��F�Y����QӘ+(ː�$�:s	$�(����^�ۈ�4K�?l��*���nu^<"p���o%S_?�/�� �c�4�9�"��wJ�-��&V����U�Xs�6�WB��{%��\~�^�@z�2�Ñ/�����|�3�B�5�L�҉�g2���5+CKSi�Mk�/�֥�'=�hc~C6OZ����ة��qk���{�~��m:���N����(|ND��	�N�F	����<�g��O^��#���>3�� �ف�/�0��x��V]o�0}�_a����Ȟ�1ik����Uk�=TU�'a���,���w�?&!�����=��\�c�4{�F�R֗Bp��yYs!I�{�2>˫��gë 9ǿ�#��K����r5M2^���ň�dM�G�/7u��H��$��{��UC��W�����8hTJ��\�����������A�f.h��B�����= �).궱����Y��&i^�JZ?jܓ���F��Y��\�UM�u��Iq����FdP��9��|Ť�:�p �oZ_�F��Q�N�zEQw3����W��uH�i�.Tu �&9�#����}'�O����v���^�G�:��DTE}�Lx�V�=�Κ�WSa�ԋun�{���+Qg6�k����WUF����vf�Z1�ՈIW��"�ܾ���m�qJ��3DM��u�N� n��ޓ�� �*�c�� E5Y�FM�p�0�x��X�~��a��'�\���G+Lk|k��U�dql�)��H�� ��K.L� 6������Fm��*�^u�`|����6<�Ŧ���qp]��Ɔ�p�F{w���i|�sji�9��'Bj7�i\8�K�$�s��XK�a�8��{,�_R�r��G�-�=�R����M�5Ahpc�d�y������^X�n�����s�ȸ��'��v�E�ÁY�	��_�����u�gT�ՂOV�ӷ �рw�'k��	(��%�Z����X���O��89p��B��A'�^>7�AҔ|�\���u�^*Z<0�	}�Sv�N��1p��F��(M��PI��ߊ��aXm��Q4�g�(y|�n��nӘ<�
q ���Y�"<[���S�jq��7�̹X0��g��$S8Ș|��9�-9/x^�ɰ�����o-��=����@g*_��-��:lw��LU8�/Z��&J�[�0W����оx�u��
�0���S,�(����=yk��E��MD���V�?x
�ff����0��(+�$b��q�f���Τ�������W,�^R''9f��$�?�9Ƃ��Pp��aw�ٗ;y�ߴ�QJk����@8�bG2�v��ib�W�z��#d��T��z�ў������Q���!�A}�������eiJf����x��x�340075UH-*�/�K�g��o�N���S-�u����e���4��)J-.��+N)��`��ɯ��c�-�옑Ц5��T I�  �Gx�uSMk�0=[�bbh�R������rH/�[E�ʻJl�H����w�������x�{��F��_�A��v赕��n�J�R��+}��l{]b��\I!�m��ӘۣP�3c�`"Ž�ֳaa����  ��)~)��-@'���򨴓���fR��K�LH3�(�.b���eIë>��?�l���(��T�x�jZ�[���݀4���$��6��
�I��r."���ڏ��i���@z��y$~U���ӗo�9	$<"���3ҍF�C��di~�
�:��-�X|�q,��� D}\�ka��6��4C�b1���Hh�"��]0�vy��r�௏����EK;�p�Nzb�r�I�/�-uWp�x^/�n�D�|ޮY���7� ���|�� G2���a��4~S�����_��hJ�u�l����<�,���5����f4_���7���o�W ���<����	����ɘ��?�zE��x��Z[sۺ~����R�H���zƝI��ڎ�rN2���$�$���5���]\H�))y�|��.�
���]2"YU��b��</�Td|x0bE"R^,g��D1�)����"W�ߒ���v��|��;�tE冦|�Os�H�T��̨b��n9K`EM��%�W05[)U��JI@܏'���T�Τ|K�k��5���;!fO�K�0$���DS��)+�Y%�2a�5����ɥP�źHch3,�`K{%٩(R��(�S������Ò�[.�
�dA�9��L�!M/�%��9���L�U�A����_
�V+!��XD!���䠉EƓ�I��֒^0���v�d�xh��,)�(�A�j��O?H0MɆ�X,Y"�(@�LN3�
���g9g�dj�I����;z�4�r{cuD��OE�c�X�ɴ���:��~�\:ume�:����0��/ן��KZ0t�f������T�Th�9�,�hU=S��_[0껴p��-��W7�]��Y,Q�(���y+�P�6�1��SZ�Bo�� p��5*"�F�[�����q�^U��)]cIn�>�Gn5�á�V@�1IXU�h�m�ۣi�QW�C�BE����c\����������߻�qCq�;m��B������Բ���\�¹�9+�j�e�I`u#�|��A��T���Q����fI���O@P���"� S;��F`�&����ۑI����1��#��a�s��Y^���J�h��[aHT��Q#��ڥ潛�ng�r��!��m���b�f���):��,�8��-aaf�N��C��6�9�׸�)�>��iV�g�IC4��YRT%��Xy_��4-�s�Xc�\Q�b�U�'��n��*`�����x^j]�c�<��Mhx�^<�#�!��nC�_���5:�_E�g����5[rHe~4w��������m�M%���6~(��U�_�j���x����� �w������,�&�VQ��ׯ��?��4?x��e�w-B� ���y���Y���*Yu�ϣx�"���]�c[�<�{������1�9���A��k��Y���|`�1����M��$�R?p��,5�D�A��Q;8��V��1�;� ֲ 1�iH:]k��F5�ΐ�x7����i���|N�5�*����?�(�h���%��s�(��l�m뉒�A�j¦��T�D�s^������H�Kx��`N��RtW��T�;sg�m�5�XC�(��@z ҏ��)}}ug��@_�ސ%�Y'a/�=Ϡ�B�l�kvT@Я�\�*sGWj7�Q����c_Pv7�<�p�@:<Hق�����B���c�8>��\�X׼�|~�	BO����SC��_ނ��lf�ŝj�E%f(v����v,���u��߃��PrR 2.�̵cD��(0��	�@�F( �1!�8>@w50��eifh/�y��ӕ�:��s;ͫ��Q�{�^1PB�M@�s�l&#�m?���3�Y��̄1"p0�|u!R����cː���Օ?1�a%%F�7?P�q�'Tn����SO�C��!�"kt�}���Fk��:fg����
�`hzJ����bgm����`�;���pD���Z�݋��&f`�^��31͠�`��'u��2!1���i�0�VS��X��x��}O��?�"t	G`�z*&[fn����Ρn�~z5r�SȠ�T.YL1L#��d����%2�2�/�5����O�.�c!K/�3)����$G���һ�^�b�����Z����["�\}4�g(� ^и�Y3ٯ�FKҧ��D��1�㕉R,��fv02��`�G��y�����i��U$w-;�#�����JV@J�ՌjHZO�6\.%MY�7�fjW爇�=��)_�bc�dq4c�"��X�tlW��,K�!/[iYh࠳y���ْf׌V��Zi�����@2��~50�u����g�h���C}���[&�f*�u?@��@7�4���}�V3z�xʈl�=���K�ߙĻS�4.B�LB�������J��l)��Tayo �q+A�=�j�X�D�o
����7M�����(�1�cL���/6qC8����;V��x��)h���y=�ąT ݦ|y��;����ӽ��&�n�6���&�������C����|\M�29cn"�����Oa�N4�wTQwc��jA��ѿx����R@��+��g�����饹д�`����2$G!n�F��S՛���J#����,e�F���������߲��K��3���4�����dn�\4���_�o����}��8���|+D֯u+K�nƇ-Jjd�7����7���50���������q�{�q����G1q$ǭ;��nP���e�|l�{,CA&d������WL�����Zr��ݯ��:�Dx[�X�?v��U��eM��oߚ{ո��u�ݼ]����x�Ʀ����;34N06��X�˜��u�y�R��'���!flYd���^Ep�ȼ�������L_,`���L&�����b9�c+���joP�����#�X3vu�͊r]`�4=�Y����a�X�WY1�ߦ�L�9�c�1�#3�<�ō�9���D��@�5��d��a��s��w�y<NꍍL6xs�/�`
�,�_A#�@˰�s����Q����=��~�_�c,
&�Ǽ�����@�ts�)�/����/��́�vhg`��K68-p��#hD��������	F6���Y#2�Tm���\�m����kX�3(P;J��ik�-#Ƥ��dc�>	�n�]|b��
��li���׼����ӣ^!cŘN�?ȳ(5����C�	Ҽ�B��o��O��:%����Պfc�Ą�A�(��ٍ�[B� �	k�ԑ7�05��x��������S<L8呂��Ry��T ��Y�P���e�n\iYf��7˷2ז�E��K���Sk�z\�m���t1Z`\:��o��O�Y�#�E��d���)��m�%'�N���zΣ�/�;�o��{��[c��
�Utg6@d�f����VI,..I��my�{	l���#��}�*��/r��^2;����){��?�m�F�/w��^��d�K����]�KqW��b��>xK��r5��{4� i�a��$J��n�磻(����˦Gm�S�m����+)|���n|��UҦ��D��l�j��:����l��ѽ%�do��A`?�/�$����H{��}�=y����h��a�~%s4����_����
�x��Zt�x�gvϼ�Ĝ̔��9�'��<��C� ���<�(EI��ӵ��" *ꜘ��_���:� ���xv6	��d��B1P�y�A!����8B�S�򀂛��.q����ϒ����(�@��Rx���=o�0@��W�� � C�!j�J]�J��b�c;��T!�{mL"J�`|~w��,q�������@���cM2�g��Z�����/����3F�L�#� �e��ͫz���"�*�A*!�b?P^��*�W.����QiyqL�9PNN)h�"�<�7�w�>1��5eI�ڮ�7����䆶�
0Ⱦp[�D�fGe'��%px���o&V
%f�9,�[������v�a�d�T��䜽0������rc�s�2_ar4#�U:g@3���(ʫ+. $�ar����3�W��fQt�2�MX���x���_T�p�Ť`o(��G=􎉱��s: ��M�f��l�6��q]J4��8oiFb���æ�?x�}SM��0=[���'L�^ȭ��eY��J)�=N�ڒ�&ۆ��ޑd'��R��i��L�?�ay>(e��yF��:�7�j�*uw��t��O��	&@c<ٞ��f6��`��7�Ź�㙏r�;+@��U|]�4���d���|l�wn�U���g�=>�����Tf��n'2���'o!T$ǋ���6�)��d���3M"�i�j���<rGH0>F�2+�b=���	z�&�9�o �G�q��Z���p��P^]�#{���:�d�L����(�]c���&I�J�[���e^���!K/����U�6��|�:\�+YV)�(��M��r�Q{q��J}L�M��P�n3཰���&A`�LbI(lk����.%�K$L�"eƻ�&�!��lY�C�Vi?�k}ձ� ���u_�i~��V�hbpv��;^ʿ�R�;�s՘���pQ�	S�<x�+HL�NLOU(�N� �9�x�343 ��ļ�̼t�֊��K�5�*uZz?��8 جߢx�31 �������ҢĒ�"�WK~%:��]�Qr־�3*sM��RRs2�R�*���Y�o}��mM�l�czV5�$?713������i���/X���[1q'C$TAI>�$��k�>��Fm�n⦝O�*k��ȦV$��d��1(W%���󍞍�v0o각���Obx���`��t�K�S�]�e%?q�����g�Q�`[d�������S��5���?wD	��$������>��ˤ�����o�&��5�^
"_Z���X��p̘��Ǟ3S�X�r^U�>0�> |��x�31 ����d�K�~���	��b��M��N]`��())`�V�}ۅ5�;Or�{�{���vD:;1-;�a�D��p�J��
%^�5\�Ss `�(��x�340031QH,-ɈO/*H�O��+)���I-�K�g��9\^(~HO�����EW?��d� �G��x�340031QH,-ɈO��M���K�gP՚�2�q�t']���fŧ��?kb 
��)�9��,e6e|?ȓ��ǻe��޾� ��F�x�340031QH,(�K�g��n<�#�2=�[�Hʫ��҃- ������ox��Ę9Acb���Ϣ���ݥ������d��� �n*�+x��̜*��@.�ٺM�9'Ep��U�	 ���x�31 �������ҢĒ�"��{�+o�8~�5]���+A�[=L��RRs2�R�*�y�����#�o{��޾g�:�J�s3��so����MLx�YA1� �+|�P%��^ƙ>v�M[j�)��U��i_�ȦV$��d��1����e���ȷ�I��R#O�/P��暗��������0v��/����g�QɰH�K����r��d�93&��p��(*I-.)f�.�t��j뢢Uƕ?d�&����/-NMN,Ne��?�29�+��Ʋ����yO ��})�x�340031Q�O,-ɈO��K�L/-J,�/�K�g���{s*dO��#��oZ����~�& �F���x��$xU�� 19;1=U!"�:{bQIfrN��L�%��9�%����y��e�J\u��'�p�O,-��OI��,K-��O/*HV��̟�C*��� $59��MM.;1-;Q��(?�49��
$���7Y��T �, 6D��Y"X�Z�_�Y�_T	�7��x�&N^�|iqjrbq�fW�`F���3X����Ŭ�@����bK��l:����g��9B�0�a����b�簋�YQ�mI��wx��*�����,��O�`���x��I_�.��ڜ��� �b�x�31 ����d�{ݯn�J�u}� �f�F����&`錒����c!��뽧���;�)�V�D:;1-;��ӧ�Χw%��=��At��� 6�(:�x�340031Q�O,-ɈO/*H�O��+)���I-�K�gX��i����mK�-��N���p �.���jx������� 19;1=U!?��$c"�^n0�%?713ob���i���`!���'DEI>TZ
*]��s�HN-(�̇�
��������!�d�^hq�sbq�&��͘�B
1τ�6ob�� i@��~x�����s���m��!���.%�
V�
j���%@�^5��>�_bn����t�R.���E�%��yV`)� X�DL�g����LSH-*RP�U���Q :��ٖ�
�?�lb�U%gQjIiQH@$	7��rj�2Ir���B��c���;=��$39'��ɍ,�"@ct@�r!�0y>�M ;GN_�x�340031Q�O,-Ɉ�())�O��+)���I-�K�g�m����4�(~�_3-�;g��S�o���(�����ƅ�5ʏ��m��h�ϑő��v(����x�ݖMlA�C���. ��Q�k�f���G��BQ�j�a�{1�2�Mqw��=4\��^ ��ģg���ћ�����ě�l[��@b����������W�(����~��sZ~��f�r9M=��v�7��0�LC��p�,�W�o�؂0>)l��*�Rҩ̺-Jo��➾X����f�@A��7Ø)�aA�&1���SI��,�,�G5G��=6���gE�<�D�.��R��0"6�]+�/�diǔq�͠G6�HC2�1p���3��@��ff��UA���A��o�j�^1� �㤣�h@�,1��|G���L	$���B�r
��틟h�T�����Wi��oh*��E����z���3���]�.����
�Ķ[U��XO@E�V��-�m%[Wz:<D�����(��sв�v����Z�5�fJ�j�ZАN_r�2�(���8�'�{��`�ؕ?$N�|˽������]޻=���6Vm����&h-tr�:K$ʼ����[@Zh�}��&p�N׀�Y'tG�!�=DN��d����h%2����#�����Ah�<N�'C�8�A9����u>��=!;�ڎ�Nu@u5�F5r`��/���Ԟ�$ʮp����&�6���"/��N���Ny�f�yw�	��j��D+He!���3Z��3��&t�pM���^������+x�;�wZo���f=fF��>�q+03���Z�\0��L\Z�y%E���ɓ��6�]� ����%Gx�;���}C/#wz~~zN�~iif��:Ƴ��%����3�KR�&�񇰄XE!��Br����h�H���u��E�:
�EE
V�
�z�ũΉũzAP�Ʌ"ʓ���q��NNd��'��眑����X\\�_��TU((��4��DS�=��-35'�M�U�`�ɦ��gMv�E7�PD�U!ѥ$�ɑ��|n�E��%(֣
aZ��h=Dh����hj1�-HI,Ij^bn*�~T!L�Ѵ ��@�G�%P��#��H,*N� �@/4��Es�A�9��HC��PDM�7?%5Gd���D;Lz?{��Ti`d� �օ�x�31 ������%�u���Ů��Ĭ/\����������D!?��$#>%?713O/=��n�d�]�»����+�4��= ̂N���x�R �����p��@0E� �k;]�I�e9< ���8aWe[����φZ�.A�͓V-�t_���0���҇�����Qٓ��9&&�x31 �������ҢĒ�"��{�+o�8~�5]���+A�[=L��RRs2�R�*�<�����p���R�ǧ$K��������<�FuU�v���	��d1�Z���@��$���Ƅ����#dMN�I�f~b�ȦV$��d��1����e���ȷ�I��R#O�/�5Y�Is��fn~}����˂o���Bd�R�3���d8�������|B1���G-�����$������ӵ����VW��Q�z��D��859�8��]j��c7����'��>��8�z� ��}"��!x�J �����u'��oW�ֆ��0l��h-�S�)�@"�������Hp� X'��/��
i��"F�i����G��$�x�31 ����d�{ݯn�J�u}� �f�F����&`錒��s��?�~^����$�g_!�ىiى�>-�p>�+�����M�k��(� �)��x�340031Q�O,-Ɉ�())�O��+)���I-�K�gX�|QXlБi�^��M�>�_���!��������U�m}b_qo*C�ɓc_��y� �,��x�31 ������%�u���Ů��Ĭ/\����������D!?��$#>%?713O/=�!z���uOC�.��Ի7sK�FUe ��ťx340031QHLNN-.�/��N��K�g�U��eх,���g��l{P�����$#5�$39�$��*%Rn��x���wyH�-�xZIi~QfUbIf~^|r~
XC����֧9��r��u���$=�TCrN&����Ԃ|��l�Z.dxދ{y��e`Η/�7BU�%�T��'�iK��O��o�7�y��0v�����P��y%E����% K��.m�^��׽��n];3<�LZ��0d4�5&�9��6h��_Pa�dUX��V�Z���k��j���y�e3z�l_i���6?�?'����v���'߷/ҕp�*)N�/ �Y��u�ߏ�7��r���ϝ��)-N-9��D{O�s��	�5�{%'N�{j �J����x�8 �����(B������m�	�?&����/W�+,�𼀜�BD#դ�@��4�kj�w.�%��Ex�}��J�@�)Mmw�M��Ŏ�BD�����E�����tl5�n�Z��I����G��'�(�=�I[��a�������K�)y3�p����&n�˷�=����Q9�������ޚ(��Yx�D%]<A����Ԋg(O��גuTzA�Z�Ձ�#�#JԎ��k�Nc'��Hl��#i2�Ej'�޴�y_Ɇ�8C)aq	�85
��e9c���Hc����kt�Ѡ�blD뉩���2]�^K���SpD+��W}���Q+6@}����ǺP��2�GR�����C�G��Jh����#�c��W�A@@H��̯W��!�"��1�Mkׂ���|�9@�{�֎n	�`j��V��@��5�&|U���z�i�V��g������'����nǨ��x�UQ�J1e�^ޤ7��Ri��҂��V-ok̆m��f)�D�,�wx����'�������y���{L�?�>��4�ff�(We�+�"�Vf�?'��q*�>��RT&� [���ʄ/�ȤܤVK��4�k��Դb� ��U!R0BM��A��x��PK�Ɏ_O��!� bm�nKM�SZ�W�5���>�f�߾0�WŬ�U�dE��#ǹ�=�k�{���� ���	+��������A��퓔ĐNKo��E�y>h���hY�g���1���ߍ1�q�aGb�(-_����'l"9/Vo�y��f�����+x��dkc��g2qB�[[��dO���K��8�;=뙊�������BzQb^I|IeAj|Abqqy~Q�^z>C���u��;���/<�2[������)�ʶ�Z�d��1�)�
�U��+[�yjTf^IQ~qAjr	Ȅ�@��Gg�m�}{aO��ƿ��(M>��29��M��piƖw��󺖦�|t4��� I�J����Kx��#}SjCkI~vj�䏬���l�d�e�\�ֵY�=�ir���\) � ���x�340075UH-*�/�K�g��o�N���S-�u����e���4��)J-.��+N)��$�^�w�����>��/�쥼D 0�(���rx�� �����9e!
�]��.�����4l100644 README.md �۰�"oO�F�	W�j`�.��I1s�r�P�t��p����:40000 db �����Q�d�+Q�J�Ѡ���?13��f��u��m���r~-,�40000 envs �?bo9";2=��c*��,�/��^&!A��>��|�J����t�V-�t_���0���҇�����Q��a���sx�P ������'c;���Al����n[j�ū��@0E� �k;]�I�e9< ��zE�6�_k?P�(m/�Q3���Aџ"��x�340031QH��K�L�K�g�~�r?S�뤶�Щ����S6g �Z�x�31 ��̊�Ң�b���+#����zq�Z��5V�&`%���E�%��y�G�o�����n����̮���| :: ��	x�340031Q��ON��K�+c����O[�j�m���Z��޲F����(?�4�$3?���\���V�;�0�Ճ�BߗL�z���$1=��rq��O��9�zl�9C���_W@������^r��B*ߡ�ִ��SR�\on� �>E��]x��̜*r��,rߣ������.�\c ��c���zx�K ������,�\]�R�c�P�/�ſ���/�^+UE�ʾ����(?O�g��u�Ib���F��/�z|�j��� �$����@x�k��;Ayb���V�Z,RZ���%��)�9zA�iE��!�٩y
Փﳇs��f�����khN���Fp��K�2�ҁ�k8R&r�s����)!��U� ��&����)x��'�/�a������-lv���ݘ6�s�g v�p��Ax�kcmc��(���%��ҿ-�N]�m��zs�# ��D�O��~x����k�@Ǳ��m�)���:�?�H��ED�к-�"Z�'E�81��d���-�,���X�U��E<{��տ@�<zs2��$�(���|�������M����Ke,b��ᗡ���\�&���.��~�C�[A(�m�Hϋl%=�f��-�1��\�U��DS�Dv�����i�f�Gz���"���E���Y�<Q�Pؗu��,p�E�t��eyx�zd��'�j�;�\�m
oKs�sZ��VE����,<2j����͘'�J-����3�҆woԴ2�R����L��H?BG9|��a�Q"(�$Ĳ�E�O]��1}ɑb�E���^V�r< ԫ�e7�r�FI^<<�����g�c��Dj	�VmS���~?�I������e-��)pT�>>�n/����S%(�R���f�R�Gr�����~�`«��{�Ʉ����NY~�m����ߧ'n���t���5�^�	�X��p���S@m��?�1�?�����LF���1S�g�l�3���y>�"���O�����x�['�H|��D�s/�s�'���$�L�`ȥ_�Z\ �*M~���&�7�4��/�6$�f�;�0}z��Y�rS6?c�a���尹�M�	,���� ?/1��4x���r�e�F�gR�7HOP��_���3\lRV���G������
x��<isǱ�~Řz�����tB���DU"T��Q��� �p��كٿ=�=�� ��v�s�R��===}O�>c�E��٬ϮDr'�Vk8�S��8J�,J�̍�aʲ�`i�'��6O�I�P�x�IĆb<,N��7�Z_}�"���2�f�C�y"�%K�q�fl<��9H��W_�Z�|�8�jx�~ȼ��"�ME(����)��S�n����\���D�,b<��]E,�=D�q���.�>��!���bW�Ë˷v��jx~r�ޟ;�?��k/هwg'�s��j9�{bח'@2��n}�|!���V��t�>����̏B�R��K�j�w	�)�e,R9�_N����F �(��I��)�r���31�/�$|��`������B���(���4]D��N�!j<@,5ʉp��Q�[�2՞� Ԇ+�i�	 ��p�'!;q��S6�nEH��N���N'�N5���0~���$ui��ۭm<�wA>ޅ��F�W �xΧH�T�8�l�Qg"!���t,��SA����_���\���j�I� �A_T+HW6M��J0O�y��}���!;�����8I.w5a
��7�o�ފ1�/VC� ^��qe��G�Q�`H�����L���h�ZϞ1#��k��Vk�e�Q��EQ�v}�M�Q2�Ͳy�K&��`��G���t���VKJ���%,�-|XP2���m���3R/ /Y�-`&��4�}%�9P*E�0��m�Hh{�5����0O����W��;V�T�΢<��X�<^Wn�\jx���(*�"�p
%ٖ���C� �",p;��3G@).Ln�����n��Q��s��  u80��W�TN���AEdh�H��(,�����˨�3�<�c`�h���]b.:@�U#����8 v c�$�%��c6N�t�	w2�"�;��F��)o��%��l�DsZ�Lהl�h4j}��׭�̨����)e�~����j�[Ku���/�r�g9�����>�4~,x�q�s�v�C�=���þo-�����?�}�����'t.z��:B,	gʊեaV/`߿z�彟��u�տ�O�Z��s��+>u�+��G\�&�ܵ�i��3?k���5���{��b��S�<�@��x<g��G~�[3�X[�3u}i{�o�6����M8�<��?�(�������	�M��t�R�~�jE!A�Z���LmT�u*�|]s&����,��mF�t��ݝ�Q,:,�@ղ4�s^R�t��b滳���z 0 !b!>��-�Ќ(���� �	�0��V�WYIBi>�����VL@O�*~/�t�{���ܨ��_j�o��?F@��vN~�	��EW|���.�	�����4��hX�����\з/�8ǉ���"�3Q�h��ˊ�~���i�����u4O���'�0ѳ(�x��q�r�CLǁ��`E�t�-iZ���^�:��5�:���^QG�H]L�*4�M���˨�8p��i���>D}�04�y�Τ�.���ۅ(��E�a�J6�zu����f�D�H�$��!���p5do/�*����XV�Yɡ¤���E�Ҳ���v_&(:e�})�)���� �E�:IW�G�fTC�����y��"�E�~3�@��H�\ݾ���"u���cI�)�U��bv.��Y-jv�P�lU�6j$�/E�<-+k>�w�=&x Gn��m��(��`%b�Y"C{���@��������å��<�,�V���d����9�}��캃}�{�s���������aY�^#�V��޹~�Gi��ېTЄo����aϊ�q"��(O�$"�?�������R4RV�&�h �hOT��أ X�c2x�g��Zn��b�0k#,�U,^%:em������;�_2ҥ~�m�lH�V��1�У�fFݴֹl�x�Rn��H���:%�ST�ó�S1�B�FA����ҵ�pL��@k�gZ���R+Ue ��P\�a��'e�ޒ�:��/��7P)��q�6�7:آt#���-��v��3����C������{�'�bg��ʙ�S�Ko|�����E���� u!9��l,\١0.p؟x^�wv{��w��ok�����<�`<8��h� U�?t��Y���4:W�>�QNsl�p/�C�����4�`I8z��-�� ��DH��K�����в1%��H�8�Oj�UH*�d��h���}	�QK��uVCY���S4���|��n��?�;~E��C�]Y�a�ߊUZ���8�(Eo��W�=�Lx�Q���&���[%2*Sr���?�{T^�p�Wq��`e~� ꨤ^�edL#_��|&��2�7Y��1�B�ν�I�6X$򈁕U�=}$��TR��vNұ�1���B�8��)P;K�����S��9E���tk�s����=1=�.;�*��s�ɹ�I�2�YG�2b�^&|���Q�[����[ͻ#��Y��W%W��.�3��=@q��4h��b���v%х�dG�$׃ϥ�S��J�ja��L���]:Z�_�.�D����Ӆ�v{�oi��?i7�)����w��-���λM"�W�_&���MO��xb��ٯ ���ڌZ�eM@��>*�1�����p���9�C1pv�c���:���� ���/���#��)�",?�1y��,BI����|�ܕ�2��ʁq�Ɋ������FL���rSڕZ�Qb7�x����������mEy�D5af��t�	dO���x֠��K(�$�gG�!{5|��rZC'_�|,<�L*�4m;��h�+;��4f�ށ�V�j-v�2Q�27�"��"�)中��T�ŋ��O�͵-���%�PR����k=`�/��]M�mIR�*���G&v$Be�+�J�>�P��:��婴ɉhR]3�,��"�q����蜤λ`�>UO=]��h8����h��w~ �p"V4+�F�*�2~���n0_D%K"D��/��+D���F��i��	i�"~P��j9WЌJT���c[� �*���B�7}^+��[a�q����R.2Jaid�{sJ��ɧ�6����܃ja�;sL��s��-Y�;<�"�́�dۘ�[�T�Lc�(3:YaU�~�t�]�!��ٲ��F)c:z��N��m��]��<��%aW������ք�R�c���U���DĪ�1/}[S���>���N���j@��K�#�+?��jM���̅�-��� -�⧿��e/�4iͅ�&<���_�K��armf�7�xt���G^�:�p�u@�q��%ފ�K��\O�+���$��&�_2i��b[y����$ͷ���L�1��T,��g�#vϒ��+�+�(��g��E�a0��DX&P �.�>s(1�0]��8X���5<z!	�:=k�|y�_xy��tM��XF*uE�-�Z�O�#���B[�	R>�����Y�W5W��S-�����o�eAs���/�y-勧Y��������xa5���}�l�~aS��Y\�������t?[��ѣL���76g���^���I�<4!xSmM��r*�ĺ�T��<�H=��%�|8E��z���Z�ը��<��U�%�ʇZ���k���I�m�S}n+�4�uc]^Ka�	���A�J-�{�]��98�*w���lN���n��-爍v�d�\f��J_A�K��T����p�?���������������#�����ϮAv��xwo|�z�Cg�G���u�#x|��c4�j�<�^R8�2����O��@N��.O��ǵ�Ɋ�����Ϯ���H��4�2����Lrz%Ȱb�i�������ɣM��&%/A	6�n5J�X�+_�e��Q���B[&ѿB����T�%w����^R��êͺ= ��u掝X�c� �O�������B�!f�4�$�lnv\� �Ȕ��|��m7� �Y������hcgi��_�������
��w�v���
��g�UV�0<�!v�2|�أ�����XTI�Og�2�D=��e/!Pc�����	d����駠/�1]l�z�Oa���Cl�w$
�,�7�)�����h4�i��a����M�%��1~8 Ц�9��~`2��l��HSĉ��tG��.�о.�4�`�R��*nfa�;���p�U�:A@�H�����w��g�F���%yx�r`�h�(!K,-6���ѩ��"Z�C*��&8���L>H~%�6�i�f�\�tߊ��l�G�h��[�:�b��Ji��b�{�г醓�����%PY]�v*d�ø�+;���z0_�����Ό�!��\G�M��3���Ө���eӈMA�=LĹ�Ch�Xn�II�n�K7R��0�-\�}�h�Q7rqܚ�[�m�s;}��[,�I3|��
,�e �ĚF牼I�e6YT-W��SvtLfk������딶�㮷>�c&��)wի��_N�eO	�K?��7>:/1�D�ϑ��&gO~ފl%��{#s㍢������G�;�Ck����#6���Ҟ�*{��~0�L3�*�5�+�(�Y3 ���[�D�`��-	!"Ҩ$>jVj���h��,�V�O^�c�����Df�{D���nn�E��T���֋%B�y@��e�
���\̣d�v�n}�i��:����nZX��~-�э��dýJ롓5֚Si ��;���B�T1��P�I�?�g4թLmk�	��tb�>V'p�mň�Xf�rD�����v����u7�)�W�`!HgEjT�Q���`��@��	>n z��ｑC�������H:�D��PP� 4f�6#����o��P�D@�y��+R�0��h�W%`jsd��}�s��E:��vc�l�g:i�ʜ5��aF���!�𻈪�e���<����By a8-Ֆ6?h��X1�SB ���d�!��I��B�`%9m�3������(V��W�Pe:#�?B�#	q�a�w�1#r~F!����7ñRKy7�#�|��0������ǘFa��ˤ�P)	,�iX��ӳ��뛓w�;���{r2�Í����Dל='�_e���s,��Py���7Jo`5���Ew�K{�;}Ղu�8�����o����7��U����o��/���W-�/��M3��@�?�E�؝�/RP�ڟ�̧�t��tdu�v���|�B�YT��bYi�LP#�����F������o��E���2�*�2�]�g����K���[^|� ������w:$5#�sFě��4��Td�Og�1��2��" Ε�$I���9��[˜EvX�Z�ܨ��H��\����w���o�B��(�.���mF#Ok&�ÿɝK�e��+�Zn�OqB������7fU�Z���P묏�ْқЀ^꧊���=u����y���],�����l�)�̖Ya���i�i%Ĵd��1�ʜ��V�W���5��,���]ަ��C_=,�^��*�1"�hDe5�e�T��]Ĝ��2G,���$
IY��jS�F�ڼ9{������jd�N��o^^�>/��t�}�����p����w7�N��S!>T�H).�hT�{���ed���Q�%t���zS	%�,$����r��N�$9&�%o+��5zS��>nf�T1�-k:�����h��>!���V��U<�ѥ6�y �j���~&FZHd�Li��wf��xȅiN�����
KF�R	���Tў�	1H �01�����N���J|/(lK��П�{M7�,�g�uub�ʗ��� |�h��G�aiA�D��`ev?.�����!sb�yrJ��iQq]�W,U�Qdy�� ܟ2zm�ی&X�=��V�v�o�~o|�[����Z2�A򳿂���}�d���iw9�z�(X�i%�iL����$�͢Sw]~�d�A{ڕ����6�[�H����ԑ���g�?�����LZ��^�AG������j�#��t��\;�w#�*N��B��.;�k�B< �Q�.�;�mS�8*�����y�~e�]���BG��ˑ0h��md�|��?x���r�eC��Ƀ�z��c�{�H���a���x �Dn��fx��̜*���`�RD卋�'>���v��� �s���x�l �����65��@�B�����s̪�.76S40000 domain ��fS��v�5���=�7��kqj�J	U�Ś=���c!�����.�ǰ��Ҫ���PaԪ����4;�x�31 ����d�{ݯn�J�u}� �f�F����&`錒��S�٪	���EFF<b���|zr=D:;1-;��ӧ�Χw%��=��At��� �'��x�340031Q�O,-Ɉ�())�O��+)���I-�K�gp��ꇫ�i.��L-�N��zl[1�嗖@���xo���{S�L��z��u � *ϫx�31 ������%�u���Ů��Ĭ/\����������D!?��$#>%?713O/=�A�Г�O'1j�]vM_r�r�� ����x� ���������0m���P�R:�W7�D몦����P���Vx�kcmc��%��`��q��	zW�>���c�ٍqL �F	k��%x���t�qb���V�� �l�)x340031Q�K�,�L��/Je�57}�����Vg?�sɰ�������(U79?77�H�e��U&��0,\�����W���GM�K�d-~]~�������ʐpu��L��#)Ju�j%ߝj)�U⛘������ (���5)���M<z_���h�U����몗����e������L�]w�����gb 
�~����m�VR���ʷ���i�d����i*����7����¢��*��qe���?������z�rU����)I��7�X/֔�%���㓣�'�@%Sr�+sS�J�^�nخx�󣋒�4�O���?U��\���x�וB���t��&�͉���N�++f�_�l��A�y�����g�_�
��(I-�K�a���a���;�zV��q�'�������������7�5m��]6^�9�[u_��@�(.�e�����3�R.��|�_����Q�\ENb^zr&$f#K}��ݚ��jz�ȵ�=k� ��̃���[.1�zEf�����>7T�?Ԭ���<��|���>W��_2}�d�OE��g΃�R��ΰ�rϢ�3�X����u5�Ujג� �9��Rx���:�u�F�gR�7HOP��_���3\lRV���G5�����OJڞ��g��7�"�{*M�@!9?/-3�����c�oN5~o8�$�?�o[DAJC�����m�gR�	h��vq������N۲�t��WW�ձ�����J�++fx��>)�R������f�g���&/f�/����2���[��?�]�w|b���Ƥ+�W���G�W�\j�����7�Pa:�x�340031QH,(�K�gHaPT9b�����w��@��g�� ��y�x�340031QH��K�L�K�g�1\du��s�燐�a^�Z,�3 �`����)x�~ �����S���0�&�2�k��'�9�&428�\6.0����
0��\ v3.2.2+incompatible�O��
�!k���a���xt v0.19�]�����x�'n��x�M��J�@����*X��,Jl�2�$�m}��d6�d�LħX���F[���'P���6r�{�w����;���K�J���Z5�P�Rܴ����p�SLHL0q,�$ϳ$f)''_�+����Ѫp�ɨ�}��!��='s����J�w.����B�e��*h�%!S���� ؞��G����틚�ڔ���f��Ju���6��3of?־�hYɥ,]�TڴZ����!�$�p���G1��3{����u:բW�����$�4�+�,%-��Dn}`�����#��g?��	�{�@d�6�?ko��J��x�U�=l�dp%-wZ���ꐮ@A���q�Ď�l��N��+N�����q>�$$$�/ F�`+�X7�����������A"l�����������O_�>{%�M��[�7/��'<p��OY��9RMq���~�>i��b'�!�7;�T/�e���|�:���\� �-
xWw��چѥ���[5��5G�dt>F��R����1�Z�O3�Bi�J@�l)�e�'KL���&M��6���VHeɌݛ�3�˺By.��K[�K��R"S�Ǡe�%���֦�� '�ϻGW!3c��y)�|)�B�0���N`�]�G2��4�eĶ�P�3���Rh���_N1�51�1M��֙A���4�H3�@��<���F�_7Sp�o�}����;z�R)�12-�#��� H� U�J<���NW��d������ALr�ڻ�x�c���M���|y�����G�Y��S��<ΰA�"?��kq8��,�4�3A&_�4�2��� @���Er����T��j���s� ���ᅒ��<ܫEDQ��js���?� p�I�Ɠ�4�����>Ln?{o��{`��}�=~y�/�+�n�<���������ӱϴJԠ��&V��l>�2!=�r�'U)�$�~s�4����)<���?�C��x�,�Vi	v.?/PTG"�[���}u�9�l �	�|sz�t�[�5����e��8DM�|$6�7v��d��u��/^Rv�#ԕ���fAW��i��E���g��Ts5h��g>�����q��0���>o����c�d.`�\����aQjU]Xu�	M�n�\��^I�E߰[ҨQ�Âߤ��enmrV���HR%1��P��&5�h�+�L�}�g)� �$�杇W�ۣ]��^1&\����8h9݁�3��9ePk,,j�G_}����S���3�O��#�T��g#Z\䋮�M�l�n��Q�p������M޾u/�R? �����!�hl�@�+�kx��лn�P �qEHH�ڊ��Qߎ$�$�Ӥ����q6_�}\�9'�'�x�n,�7 E�7�Ix j��"U�姿�_?[_~���xy���ݛc���p�*�6��\!�}�������ލG�+Mʠ7�Q��Z�����Ӱ��u�p�.N� � ��#�a��5�׏;}b�rES��.ys$Y^��P45��b��������G��ql�t��G!l����Ȝ�+�T�R 2B2΄M��%r�_^+R��%����|�=����c�pܛ�nXgKyl��L�	�Z�:T�G�ۉ���,�۬����Fdd�	�C0�DP��G�R�fF����y0�E�8촆\=g��*3:��w
�"�+��s�uL��:�Մ��+�!��^
���w���Q�W0��t�����Q�|�<�L"_V2G
 ��,*L�y�t��̺�����Ig������c�h����B%�V��N6g�Sg,��V�5��ۏ�c�����jx�����#W�u�"" 
KD
�\eWB��9�H��y��ǌ=��3ͼ=��{�n�h��Qҁ�DEC����7l�� ��{��7AY�N{����=��z��Ϯ������[[#��q�B^���۲ʝ⦆oO�9 � ��9e��m��E;��|��U��=��GM�����v�+#HW��h7���j4���z~���[+����zo,{�~%�Y��F�e?���E�L��6϶��*7�n���W�]�N�~�@�����N���$��yBLy�TV��P�@�Js�s&ӗT��#EL\2
hh�LDc}��.���m��j���vB������n�f�7kV��y��~�/�J+��(����F"#�j�F��-�yY����y!*�2[�k�\�F��P�$�0�\��#��Gw�}���?�����N�f�@QRN�������H��V�tZ�$��1�"Z�	O�J�jH�Y�ҥE=����'��+�u*���F,e6*�c��ڈ�M܂��3���"#7������o�������ku37�fS�.�����/|�b�f��Z�!['��w~\�W���v��o5@�)<���i�7uX3xjL��S,X���&��?�X���u�ˇ���p4��t��I�o��4�(����)���-��LM.�6g��O��JL¬#�ٞ�`���b��l5Z�*E�6�B�3��B��$<>�����_�:1����I^��{�# E B��N8��Y�@Dt� |4�B<�B�����R��a
���$_�%�?(��曉^�¶����5)Y��2�I����̗��6�b��k����?y����	T�~��n�3e��u��*~����6{;�}�΀�SH�e�f�zA�5�e)$tQ�����b�M�H��Kf�M�Ģ8w9�+v�Z-E?y��񝧿9�UQ�IQ@fhX�����}9��A�{J�z=ޏV΂�O�]�i~7�t�eW{�E��^>�7��M�p�y❖fa9T�75z�[p8�ML�F2�{5샐��2�$�ԁ�f�l�1�	����7@_xW�Y���S;ѳ�P�)�#���VM1�UO�ys�_<���oO??���������CinJ� �m.��F��	�HB�#�b>���8>������ᤚ$�?�vC)��`�i�8�ɹIbѪ�X���]�.@O�a��yQ��Lsp�n�6�o
w1��0�r�,9��� ٌp�<���5��Ͱ��v�h���t'=���LfDu�3���%4�h��@S74��??��]o%�2;F��2���C�^���0��iPn\C#��܅ǩ��b<R�=�n��Ff&&l.7���vJ�ܡ��f��?>Ӟ����`����Ac�����V����S���L�R\]�!3��3�ץj��#,�G��0�x������y7�߽>�·Q��G���A���΀���z8o?����K�C���m�`}��o����"���G �I�i���ӟ��8W"��.E�1<)=�R�ѝ3x��
�t�i�;�V��G�^ó:��u�ӑ+�=��bz�ԭ�w]B�K��:&i���1�Qx[��\�f@���:j�q]B;f_)/8�%6���ޚV1��Ǿ�����ϯ� �t+����!x��̜*��b��s2aW��s�Б( �R�'x��̜*R�Ƴ%�������[��m9�; �����ax� ����������7E��d9�lé�R����"Aq�x�31 ��̊�Ң�b��]ZFW:���\�v��a��35^���d敤�%�d��3�6��j��}-����n����� wf !�x�340031Q�O,-Ɉ��+IM/J,��ϋOˬ()-J�K�ghoSv)j	r�?M�h�Y�,��g 2/��x��VKo�F��WL�Ccה����(�Z�c ��)��"G���.��������.%YΡz�jޏofx�nu�,�i0ˊ��వ^랡�&(m<�5���+�h��:�����bYvv�[����p�TPa��gpv�� �m��a�<��.џ�e�/?�9̋Ѭ�ʖ��P�AG�+(mPy�GĖ��l�X�CT��'P=������hq|7����������擫��nz7�y1�a6�g_�����܏G�D,d9�1Z�.H�)�o�N�r��vʒ�B���!��(FS"�]��5���$�(G�ב�'�i@r�����45�x{�?��1t-;�_�^P���B|%���$z_�.M��ez&���DN��=ǲ��vnL��z��a�#�{��5�1c%�أ�t��<;9�Wc��NVB{��i���0٣FMsD�a�xe۔���<hä��0�+�������F9��DX:���@�'U@Sk��X)��$�����SD�)/1(���:;�	z�*�%�p^�q�dou�5����r���=cU����u��w/�um`�%�3d��k�K� !��
������ۊKV�k{q�w�v�ǽ�ֶ�W���vֈ���i�lv�h�"��8�3������B�N{?�p�ӷw�Z�q8� XW��b���F2qY��Z�X��Ãʙ��57�p&lkT�Q�XĮ!�1��uj��:@)Fw�����a�u�V�YU�"�e��;��dVV���Ƴ��@�JY�kj'������ߨG�E���:�����͍�X�u恤����'MΤ�b�3J�`���?_\\H�jז��Kb��TJ�����z����3��3q�(���f�B�.�g�W�?@f� �&�K
�rb��?�U!���Dd�8y�O���%tҡ�TA���>���>0-rqƫ<�
�`���{��3@�V�$R�ڔMǗ�? �zc[>�[�$�����g�;����Eb��"�-�!�!\Q=�7mp����tA'`H��~��0�;�T�h��[���@%-9�Ѩ$H��+�wC���j\���J��n1O�Q�N���E�<r/9��Y��G�:��K�b�7n:�ߔ����-}�D$�W�nt��Â�МC�Z#،�P��iK���z������!>d�J�/@��x�340031QH��K�L�K�ghӜ�aU]طs��zUX���� ��ƨx�31 ��̊�Ң�b���+#����zq�Z��5V�&`%���E�%��y�~Yl���%�릭�+^ >����Kx�� t�����μ��8�P�S��0�}��`����w�~4XU��.���.�����A�P,���b��l�|��T���B��f�sE�f@��9�-����*�pN��������Ie�ѹqk��xZ�A��x�� t�������Z
��8�F>�f�~�e���`�1�F����	�})洎�,����A�7��M1�[<�U�m��'����B��i��:��WGgt7��=��*�pN�4�5�Y�k���S'=���*�ɀ>ro��~x�����}������;W0 /��f�#x��ø�q�6 	�ۨ	x�340031Q��ON��K�+c�ý��ކ=F7�r�&���c��	�EE�)��%��y`���u�v5����|=�)���1��P��%��`E�_��`���zy�W��Չ�g�@������~�}+�i�}þ������GT  .�@����?x�{��e�kN~rb�F�����ی
P�Fk(ˎI ������lx�{��e�KAQ~�F���`��ی
�FkÎI W�R���x�{��e�(���BqIbz�F����`��ی
P�Fk(ˎI ��W�x�31 �������ҢĒ�"�F�8�w�dP��'Ɯ#<Mj�	XYJjNfYjQ%���S����/<��t�Y�W`�O������<-Ӻm�U����yf��'�n����$�!i�:?��DK�[g����e�'"�Z��ZP�����z)�\Uõ7|?(̘���m���!j��暗��������0v��/����g�Qɰ���
ͧ�k�M�����E�ODQIjqI1���3����M�ܙsx��砦�!�ũɉũ��Nkkn��n�[����&�  �����#x��Ĵ���� ����2�K�K�V�u��������u���r�%o��(KI��,K-�d�1k��櫿��M�8�������.����=߼z�����?r[ڗ6�� �/Ӯx�31 ����d�{ݯn�J�u}� �f�F����&`錒���k�έ>�7)�OZ�:��gsD:;1-;��ӧ�Χw%��=��At��� �'Y�x�340031Q�O,-Ɉ�())�O��+)���I-�K�g�s	��Z�P��6�}W���2��S�_ZQoc��t���+�
�Og�Ǯ��� i�'���� x�����yC	�x��{Q~i��R~biI��&g5'g�䙌����1�sq�rq�r �> �x�31 �����������?y�+{Vo�ǈ�ZC3���Ғ�������<��|�ć�;��&�m����>���V% �j�x�340031Q�O,-ɈO)��K�g8������u��w���V
.R�`�6��+-N-*��3->����=��j�?��~�� �Jͼ�x��Qs�6���Py�1bޙ��Z��@�+���ؖ+�wM:��ݕl,�2���!�����'i�<���%<,�~.��4�B���z%���4!����K�t�gv��K�Ɏߦ,�V�4OBI',�Tda2Q�Ob��,��hl��P�����	�CɅh��W>�?��
�Y����]B'e�b�H�ҡ7�<��S�39��`���{�qI�.i!!!����$��(�G|��(�v�鯂g��%�7|�h�T���5�4f�Fr-XKCT�M)���0����aKI60�'���eGnNMhD���ն13�z����MS�@�ƙ*K��!�`�i
S��P8X̧�v4��7������3��SW�v�@��hB=��/�T�C����h��%�F=C���8<:aC��u��VF���?¢��E|��c��\�)mWV�S�i���8�"�S�*(p�Em7���k�Z-�R4��^`���&�c[����������K�Ѡk�:JW����Yҭ����?���q~�R�V��D���Yd���J��B�a0�W
��O�p�I���r'��*71��N�7h�]J����-�\ޯ/w�5(���u)Vu��BM��~���2��
���`�e	�/Ƅ����b��N�.S!b���T���;����y�L�WI����grJ��<p���{-GW�w�`�B�,�$���r�Uj@T��B��|V/k�ub\��Y�jp^
u��i��i��x+Sj�kgE��߼���rG2�`J����ں;:-��&�LV8�f:�̢�ŏ����s��-Z��a9#��bX�Ҿ,`�/���(=m�=�HG�'�.�t�*�gUIy�����G.�oٔg��(^�U�V�h2!Ʋ^�"�YAI��q	�������`z6,��7>����״�5�M���#<��S�V��0�i�i�W�ޞ��;�5-�F��ig�!k�8�$����y�L3T�'/�v���=�m�S߀l�&�}y���w�H ��>���!����\['mo�4&	�R\iz�{ڞ��7� ƺS�ͯ
OO�.�m����ٛ��?��:����]Ĉ���M�+��H���zgsV9��nS۳�k�I�)kY�Y�z�vWT'� �<3��3x��RMK�@=���!I�M+ҋ7�
�Q��dLW�������MbM�Xq��2o^-�GQ �pv9�Ĺ�j�bΞE)saICTH�t�4�jTАV+�g�R$���>��G��<�	���F8�(,�js�O���X�2o�D�
CJ����I��t4���}
�t���1/��-�nӞ����Sę��*��f3l�SmeV�G�Pk���W�uZ��Ni���z��À�����2��2v�F�n�@���H��V�
Ua��� &�$`�/��z���d G���wb��~��S�2ط����Jg3�ݞW������ڰ~4ц�������x�340031Q�O,-ɈO�HN-(����K�g(��v�8U�l[Õ���p?^-hQ]Z�ZT��:���='��A>�����pu��4 ]#��x�340031QHLNN-.�/��N��K�gX�ڑ=_ӿ�I�������_rDB���d��d&'������&�|I���͙;m9�'����̪Ē���������:��ߛe/�¿��
	�
/���L���E�� �}�˗~z�1��׳����|~/c U�^�TXRY��ݖ��o��4�L�^^S,r͗8���73��(�� 5���]Ta��㍼Ǖc|Ou��C惌���[Q;�A��;��Sg��N���L�*,JM+J-�@�_ۭ�y�-�.E���	�a=�aS��v`���'����xI5���Ļz6gC�'���w����%5��jJ��O��s
�*,-N-)��KrR�&�8)��^�֛B��� ٖ���&��x���1o�@ǕЦ�	���*�0B��J7��QE��Hi
�QGc.���szw.FUՙ��1vd�+0�)� H`;i;��v����w��o_������=�V�X�鴶t��#���㩲���F��F:����PB��9++�F���8��s4l���0� C�:�V[`�泲��A�Fp��6��x"��'a?N��W/��@e���O�����n��u�9���z}��6�N�W8�*3�c1�� zϻ]�g�1sv�m��J�U�2s +lyq�]����K�.9�S�$8'�Z����|�
6̑gE�v��2' ��fY�{g64�yG��݅s�Q�����Ɲ������P���g��q��_��� �Gkx%�n�;��}��}��I;�@bFRc��2�J���.���D��[�*�9�H��� ]*�R�)��Ex�u��J#A�!1MW��?hP��D:Z�a M�L�(DEp�Mw��;VWb�a�pa��,������l|�V��kQn}�V�S���w����D�=W�.R�t9�,/�͹6�Z�W�p扃��iR2�������4
= x� ��T^!go1�o4�yă߮���lt�J)<`McG_GMϮ���Dz.�JRqW�eJ���}j����M��%ʝ�y`'��[r������OP"���2������PfB���c��.j���7tfv0�6�.`=���������/�$�~s5�H���jK%��ۍ�b5&X_.�w�*�E�
ơ3Ke��P}op,�I�Y1�U�CI|QK�*�
�e�����d:������0��%N(9.��&���<�c>e&��HIbPa��;1��{���4x�{�1�cB�ă�ؓs2S�J�'�fܱ9�i?�d[ ҹf��x��UM��6=K�b*��U��l�f�t��S�W��ҤLRk;���?d[���P��es8�f�{��7|��y��3l�N�C��M���2ϊF+�{W�#���O�N÷3t���Z�)��Gn��/`j̍z�R,�n�γ����[ܕE:��
��Q����`������\-*K�'�iݡ��e����t>W���v}�p�r��¨@��T�i�9N�q���C�U}��S��=ΈA��7�ìf�9��� D�3�(`Ti2���y�80mMP_ǼGĬ%d�=����B��Y	w��[��K���:䝆h�}�? q�Q��aNy�(����^%�-��F�#�S]�4�%R���߿�:��!�ig��D�t�zX�~�xf��}�� ������������>��o��)	�>���$7��A]�:C��ʠ��[����˾F�t�����F ��%gI�@����N��u���d`-�N�'4#hh�#����X��3�!R?�q��R��[�b�r�>�G�;������~��Z�.)?��s�w��M��-�EW;�u�7��2�sٮ�=:�p)ɜ~W��S��޶���(�F�L׷�P��Z���:")��cT����Yc�`���C��G��v��L���R����o*��(��
w��R��Fnh��6��iB�)k��W�������W�m����C��]0Xse���\�|1�N�5��h\k�@C�&�!'��wG68��z����+�ؓ�X���juԕ��y���i�xV���p}�4/��Ka*4�:�M�oС�iN��6�5�s5��<�BFEVDn�{l:�s9V5h����	���i�w�6�4����_UK.�&���K����$)���� -�6�����Qr�6y$��%x�340031QH��L�+�/-NMN,N�K�gHw��p���Y�޻1/�F��*&C����D�ڒʂ���Ғ���̪Ē���������M�v�\��DN�$^z�l^���jgrQj
��L�)��f�΍e�7}���~�p�L������E) �*����3��=�=����N�?SGQjZQjqF|I~vjH[��>��ẇuʪ�o���jރj��+)�/.HM.)��9�5��Q�֮����L���*��O��|�u��)�.\ڰmƞ�b��׻�j�Aa��F��+/�e-؝�o3s���
g@�b8���b�b���f핰vO�4Lm~N*8�ܮ�f-y#�!t
�y\���Cz�P5���@�#���P����gۥ-��>kF���Ԧ#�j�Յ
��q�^����ߺkU��F�*�YE��&:��(u�|����j����%�}� � �}��Hx���r�eB�H��VI�x?���	��kx���m�|� Ɯ��5��Kx�mRMo1U���q��@i��ipķTQ�d(R)(�G˙M�$��녶�zGB�� ��
�����Hm�^�{�����u����)�c6DP,7��c�!!b�*m $A�+ip��]�Z+��(���ٗ���N�p��}��q6g7��y{Pzr�Z�P�|�&b�ǁ���n��R�~����a}Q zQ�����$�C#/XG�v��%cuu��5�&�f2h\�M��*�Eۜc��Pc�}�R%3l"xC���~��JĖ�zw~�[�s�1U�0J��Gh�h��}�z�&��@�p���eZ�I!!�H����3��%�O/�m
�jB�!8u��e�5#ј���JȐ������?�.�s����[����K�
]��A�������(8!.t&��S-\�Ob����S�~x������2�x�NF��%uK3����A9?�2+gѥ��x{���£��La;Mi>���b=���HЈ)���մ�L�v�ըk4��p*m)&��#&���x������p��5��BGa�;& =����Bx��ʿ�c�.#���,��k�&�a��qO-Q(�HU(N�/ �%E�y�\�`��BjQ����Bi�PY0HL#��B�X�k�D6������R���S��J�S�8!��L��E�y%�`nHN���39'35�D�����Q��WH�-(�T(-N-��b��(�,J-V�̛<�ySf�d�� ��Ax�	��Yx��<+0A�C)��(��Xi�\{�&WYb������kQ�sb^^~Ipj�knAIehqjQ^bn����'ni[��z~��JE
ũ%
� e
�PuJ�\@���4�Vd�l|W�:����-�� �6����=x�! �����6ڟ�yO䊞�0��D��
�iF�JG��k#�a�x�340031QH��+.I�+�K�gXpt��3�nY�d��?���J���&@��ZT�_İy��`��+���3*��b���!�9���E)���0�2y#�H��W�K��U ��,��	�ex�� b�����;J/��HJa���f~̿�F�*:(�G�Ǝ	�ג4� k��-��40000 deployments c�/�=�&z�m����E~ݓeo6�Ҟ�o��u��>������g100644 go.sum zr��I|Zo7-9�wb-T�
L�yGAJ�	�,x�� d�����T<��$a��w����8s�r�*�ǃ���c�g���ŉ�p ܓ?�6�bEf���<J�	�-�L�X100644 go.sum ������ d�G,�_�n��
L#100644 main.go �W��S{Es������r�w���V�B�f��bx��Ș1Q3 ~�x�340031Q�M���K�g���vj��T�5.��w�2%�o�  �W��x�}��� �g�S`�v(�&]����H)=�[b��ݥ�������h4wH�\)�X3��T1��r*89ҕ��G�&�T
}�.Z�Zb�����I�ѩ�`*^}vێ����v�F����ıT�&����=����5qH��mE�þ��Q���T�;�h|��G�J��x�31 �����Ē���b�	\���Dd�:��â�9vA�ڵ �f���px�� ?�����r853�D�Ё8�i2+D&Q�n��/LX�m�U������'m��$db �A끥=�Cp|�5�΢�N�?�6�~?���_l?F�B��"100644 go.sum ��{�z��@6�o}7*�B?.��
82I�| rP:��������~��40000 pkg ��Wg�k0`����� ��!+��RY��Ox�� 6�����N�3j¶K"$L��M�ﾖ��$8config �"�v����J�RI�m6F40000 db (�G�Ǝ	�ג4� k��-�ޓ�6�Z �r7�K����$)��v100644 go.sum 4�T%!��uL��@6�B���82�n�0�XW�ܼ���Y �}i40000 pkg �ӌ���n����[���1+T����ax���tCqC,��ٌ�Ɖ)��yV`�!'?91'#�������H?=?>73�(�8��,395�$5� '�$u�F}~ hM��x�340031QH��K�L�K�gп�r�Y����/���87^���B �u���fx���;�wB���3��2�Nf꘼��ub��� V���
&/dΙ��(��$M���f�L���Q �fd=1y6��u��� �i�Vx��ƻ�wB�da&щn����2sr��S��3����L�&0+N.`���(3y3�dƩ��Y�&�c���T ז\Z�������
�j8y"���L�����6_fna���*� ��&�� �px��пN�@ ��u��;�@ယ8�*��UZl7�\9zWK_���7�h�8�(J\��8��O����n��l<nn=�n�C<�W��B�7��~$.FKY�Fѵ���DϜ�C� �s��p�v�V����p��,ʽ��S>�8D���}�z���ZHh[)&<�:�X6�D����"Q����w�p���=�q#���y}��V�[qN�k��,S�LQE��(�."6sT)j'��\k��U��~�=��׺�����r8���{%��1��\�X�$x�S�Oi�g���_�u.�cep�`���5�Q�l���������"'���r���{˦�EH%|9Z����V��I�S_E� N�j�9��Za�����Rx��̜*"�U��}�S�𞆕	7�㍺'m ��#�x�340031Q�O,-ɈO��K�L/-J,�/�K�gHs�����>��{`��I]�9O� ���x�31 ����d���C���T�q�y����G�L��%%���^�[}�oR����uf��
�tvbZv"çO�-�O�J�{0v����'�k_'��x�340031Q�O,-ɈO/*H�O��+)���I-�K�gH�����m�������^}Fy� �'�x�340031QH,(�K�g�>���/Kٻ��g��kR�U8� �}3�x�31 �����Ē���b�ͫ-�xֻ�]`��׶�o�� gZe��7x���^7� Sfd��Cx�����	 ���x�340031Q(-��)�K�g8x��k�}�e�u�f[�Y����� ��ߏe��,��V��l1Zn�n�

// .git/objects/pack/pack-8f65ebed1d2cf1e556abe96c315a170d6eb96e9b.rev
RIDX        �   �  �  �          �   �   e  |  �   �   }   w  �   D   �  Z   �   "      (        �     ;   �   �  ~    �   Z     T   �   c    +   <   �   �   `     q  �  v   �   �  �  �   3  �  �  �  �     ^  T  i   2  �   A        d   �  g  !  �   �  �  ?  �  �   �  �   �   �  �    �  �   �  G  �  �  �  �  a     �  �  �     �   f   �   �   _  "    �  �   �  �     $  N  �  �   �  t   �  s      -   �        �  ]   �  �  B  "   �   �    �  8  �   �         �      �   Q  �         0  &  �  �     I   �  O  }  Y  A  -    �  Q  �    3   ]  �   �  x   *  �  H   j  �   �  4     a  W  q   �  ^  f   ;     E   �   �  �   N  �   �   �        l   t   &   b  ,  <      H  �  =   �  �   �  @   J   �   �  �  6   �  �  )   C  /    
   O  $  �   �     �  �  �  �  �   i   X   �   \   �   �  �   �      o  �      �    %  �   :      �  j      �   V   L   �     �  �  *  �   �   �  �   W   �  �   8    D  �   �   �     �   �  �  �     �       p  E   9  '  b   �    �   )  �     2   P  �  `   +   6   U  �   �   �       �     G    C     �   
  	   �  >  P   �     \   ?   R     �   x   �  
  5  r      �   �  �  �   �    �   Y  V   �  o   @  �   ,      !     1   �   �   �  �   �   �   �   #     g  .  1  �     L  �  �     �  $   �  �   �  e   �   �   ~  U     |   �  �  0   u    J   �  X  �  {      p   �   �   �     r  _   �  �   [   �   �  �  �  &   y  �   n   5  �   F  �  h   �     '   �   K  �  �   �  �  �  �  z  �  y   �  �   =   /   �  �     k   s   >  �  k   �   7  S  �   �  m     �  n     �  �  �       �    �  u     �  9   �  �   �   4          �   �  �  �     v  �    �   M  �     �  �   	   �  d  �  �  �   �  '  �     �  	  #  �   �   �  %  �  (  �  �  :   �  R     7  (   �   �   .  w  F   %   �      �  �   m     c  [       M   B   �  �   �  I   �  �  �   �   z   �   {  #   S   �     �  �   !  �   l            �   �  �   h   �  K�e��,��V��l1Zn�n�|Kp�9�d�{5�t
��?(�

// .git/packed-refs
# pack-refs with: peeled fully-peeled sorted 
c06b82170e29a4eb519c9cff2392b3a8285e5306 refs/remotes/origin/main


// .git/refs/heads/main
c06b82170e29a4eb519c9cff2392b3a8285e5306


// .git/refs/remotes/origin/HEAD
ref: refs/remotes/origin/main


// LICENSE
MIT License

Copyright (c) 2022 Diki Haryadi

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.


// Makefile
PKG := github.com/diki-haryadi/go-micro-template
VERSION ?= $(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
BINARY_NAME=infranyx_go_grpc_template
BINARY_PATH=./out/bin/$(BINARY_NAME)
MAIN_PATH=./cmd/main.go

GOCMD=go

TEST_COVERAGE_FLAGS = -race -coverprofile=coverage.out -covermode=atomic
TEST_FLAGS?= -timeout 15m

# Set ENV
include ./envs/.env
export $(shell sed 's/=.*//' ./envs/.env)
export PG_URL=postgres://$(PG_USER):$(PG_PASS)@$(PG_HOST):$(PG_PORT)/$(PG_DB)?sslmode=disable ### DB Conn String For Migrations

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

## ---------- Usual ----------
.PHONY: vendor
vendor: ## go mod vendor
	$(GOCMD) mod vendor

.PHONY: tidy
tidy: ## go mod tidy
	$(GOCMD) mod tidy

.PHONY: vet
vet: ## go mod vet
	$(GOCMD) vet

.PHONY: dep
dep: ## go mod download
	$(GOCMD) mod download

.PHONY: run_dev
run_dev: ## go run cmd/main.go
	$(GOCMD) run $(MAIN_PATH)

.PHONY: watch
watch: ## Run the code with cosmtrek/air to have automatic reload on changes
	$(eval PACKAGE_NAME=$(shell head -n 1 go.mod | cut -d ' ' -f2))
	docker run -it --rm -w /go/src/$(PACKAGE_NAME) -v $(shell pwd):/go/src/$(PACKAGE_NAME) -p $(SERVICE_PORT):$(SERVICE_PORT) cosmtrek/air

## ---------- Build ----------
.PHONY: build
build: tidy vendor ## tidy , vendor , mkdir out/bin , build
	mkdir -p out/bin

	GOARCH=amd64 GOOS=darwin GO111MODULE=on $(GOCMD) build -mod vendor -o  ${BINARY_PATH}  ${MAIN_PATH}

.PHONY: run
run: ## run binary
	GOARCH=amd64 GOOS=darwin ./${BINARY_PATH}

.PHONY: clean
clean: ## Remove build related file
	go clean
	rm -fr ./bin
	rm -fr ./out
	rm -f ./junit-report.xml checkstyle-report.xml ./coverage.xml ./coverage.out ./profile.cov yamllint-checkstyle.xml

## ---------- Test ----------
.PHONY: test
test: ## go clean -testcache && go test ./...
	go clean -testcache && go test ./...

.PHONY: test_coverage
test_coverage: ## go test ./... -coverprofile=coverage.out
	go test ./... -coverprofile=coverage.out

## ---------- Lint ----------
.PHONY: lint
lint: lint-go lint-dockerfile  ## Run all available linters

.PHONY: lint-dockerfile
lint-dockerfile: ## Lint your Dockerfile
# If dockerfile is present we lint it.
ifeq ($(shell test -e ./Dockerfile && echo -n yes),yes)
	$(eval CONFIG_OPTION = $(shell [ -e $(shell pwd)/.hadolint.yaml ] && echo "-v $(shell pwd)/.hadolint.yaml:/root/.config/hadolint.yaml" || echo "" ))
	$(eval OUTPUT_OPTIONS = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "--format checkstyle" || echo "" ))
	$(eval OUTPUT_FILE = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "| tee /dev/tty > checkstyle-report.xml" || echo "" ))
	docker run --rm -i $(CONFIG_OPTION) hadolint/hadolint hadolint $(OUTPUT_OPTIONS) - < ./Dockerfile $(OUTPUT_FILE)
endif

.PHONY: lint-go
lint-go: ## Use golintci-lint on your project
	$(eval OUTPUT_OPTIONS = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "--out-format checkstyle ./... | tee /dev/tty > checkstyle-report.xml" || echo "" ))
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:latest-alpine golangci-lint run --deadline=65s $(OUTPUT_OPTIONS)

## ---------- Migration ----------
.PHONY: rollback
migrate-rollback: ### migration roll-back
	migrate -source db/migrations -database $(PG_URL) down

.PHONY: drop
migrate-drop: ### migration drop
	migrate -source db/migrations -database $(PG_URL)  drop

.PHONY: migrate-create
migrate-create:  ### create new migration
	migrate create -ext sql -dir db/migrations $(migrate_name)

.PHONY: migrate-up
migrate-up: ### migration up
	migrate -path db/migrations -database $(PG_URL) up

.PHONY: force
migrate-force: ### force
	migrate -path db/migrations -database $(PG_URL) force $(id)

.PHONY: compose-up
compose-up: ## docker-compose up
	docker-compose -f deployments/docker-compose.yaml up -d

.PHONY: seed
seed: ## docker-compose up
	go run main.go load_data

## ---------- Help ----------
.PHONY: help
help: ## Show this help.
	@echo ''
	@echo ${CYAN}'PKG:' ${GREEN}$(PKG)
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

// README.md
# OAuth2 Server Documentation

A comprehensive OAuth2 authorization server implementation supporting multiple grant types and complete user, client, and token management.

## Features

### OAuth2 Grant Types Support
- Authorization Code Grant
- Client Credentials Grant
- Password Grant (Resource Owner Password Credentials)
- Refresh Token Grant

### Client Management
- Client registration and authentication
- Multiple redirect URIs support
- Grant type restrictions
- Scope-based access control
- Confidential and public client support

### User Management
- User registration and authentication
- Role-based access control
- Password reset functionality
- Email verification
- Profile management

### Token Management
- Access token generation and validation
- Refresh token handling
- Token introspection
- Token revocation (single and bulk)
- Active session management

### Security Features
- Rate limiting
- Audit logging
- Session management
- Basic authentication support
- Scope-based authorization
- Token introspection

### Consent Management
- User consent tracking
- Granular permission control
- Consent revocation
- Consent history

## API Documentation

### OAuth2 Endpoints

#### Token Endpoint
```http
POST /api/v1/oauth/tokens
Authorization: Basic {client_credentials}
Content-Type: application/x-www-form-urlencoded
```

Supported grant types:
1. Authorization Code
```
grant_type=authorization_code
code={authorization_code}
redirect_uri={redirect_uri}
```

2. Client Credentials
```
grant_type=client_credentials
scope={scope}
```

3. Password
```
grant_type=password
username={username}
password={password}
scope={scope}
```

4. Refresh Token
```
grant_type=refresh_token
refresh_token={refresh_token}
```

#### Token Introspection
```http
POST /api/v1/oauth/introspect
Authorization: Basic {client_credentials}
Content-Type: application/x-www-form-urlencoded

token={token}
token_type_hint={access_token|refresh_token}
```

### Client Management API

#### Register New Client
```http
POST /api/v1/oauth/clients
Content-Type: application/json

{
    "name": "My Application",
    "redirect_uris": ["https://app.example.com/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "scope": "read write",
    "confidential": true
}
```

#### List Clients
```http
GET /api/v1/oauth/clients
```

#### Update Client
```http
PUT /api/v1/oauth/clients/{client_id}
```

#### Delete Client
```http
DELETE /api/v1/oauth/clients/{client_id}
```

### User Management API

#### Register User
```http
POST /api/v1/users
Content-Type: application/json

{
    "username": "john.doe@example.com",
    "password": "secure_password",
    "name": "John Doe",
    "roles": ["user"]
}
```

#### User Operations
```http
PUT /api/v1/users/profile          # Update user profile
POST /api/v1/users/reset-password  # Reset password
POST /api/v1/users/verify-email    # Verify email
GET /api/v1/users                  # List users (admin only)
PUT /api/v1/users/{user_id}        # Update user (admin only)
DELETE /api/v1/users/{user_id}     # Delete user (admin only)
```

### Scope Management API

#### Create Scope
```http
POST /api/v1/oauth/scopes
Content-Type: application/json

{
    "name": "read_profile",
    "description": "Read user profile information"
}
```

#### Scope Operations
```http
GET /api/v1/oauth/scopes           # List scopes
PUT /api/v1/oauth/scopes/{scope_id}      # Update scope
DELETE /api/v1/oauth/scopes/{scope_id}    # Delete scope
```

### Consent Management API

#### Create Consent
```http
POST /api/v1/oauth/consents
Content-Type: application/json

{
    "client_id": "client_id",
    "scopes": ["read", "write"]
}
```

#### Consent Operations
```http
GET /api/v1/oauth/consents                # List consents
DELETE /api/v1/oauth/consents/{consent_id} # Revoke consent
```

### Security & Monitoring API

#### Token Management
```http
GET /api/v1/oauth/tokens          # List active tokens
POST /api/v1/oauth/tokens/revoke  # Revoke specific token
POST /api/v1/oauth/tokens/bulk-revoke # Bulk revoke tokens
```

#### Monitoring
```http
GET /api/v1/audit-logs           # View audit logs
GET /api/v1/oauth/rate-limits    # Check rate limit status
GET /api/v1/oauth/sessions       # List active sessions
DELETE /api/v1/oauth/sessions/{session_id} # End session
```

## Authentication

Most endpoints require authentication using HTTP Basic Authentication with client credentials:
```
Authorization: Basic base64(client_id:client_secret)
```

## Error Handling

The API uses standard HTTP status codes and returns errors in the following format:
```json
{
    "error": "error_code",
    "error_description": "Detailed error message",
    "error_uri": "https://documentation/errors/error_code"
}
```

## Rate Limiting

The API implements rate limiting per client. Current limits can be checked via the rate-limiting endpoint.

## Security Considerations

1. Always use HTTPS in production
2. Implement proper password hashing
3. Store client secrets securely
4. Implement token encryption
5. Set up proper CORS configuration
6. Enable audit logging
7. Implement IP whitelisting where appropriate

## Database Schema

The server requires the following database tables:
- users
- clients
- access_tokens
- refresh_tokens
- authorization_codes
- scopes
- client_scopes
- user_consents
- audit_logs

// app/app.go
package app

import (
	"context"
	authConfigurator "github.com/diki-haryadi/go-micro-template/internal/authentication/configurator"
	oauthConfigurator "github.com/diki-haryadi/go-micro-template/internal/oauth/configurator"
	"github.com/diki-haryadi/ztools/config"
	"github.com/diki-haryadi/ztools/env"
	"github.com/diki-haryadi/ztools/logger"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	articleConfigurator "github.com/diki-haryadi/go-micro-template/internal/article/configurator"
	healthCheckConfigurator "github.com/diki-haryadi/go-micro-template/internal/health_check/configurator"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	iContainer "github.com/diki-haryadi/ztools/infra_container"
)

type App struct{}

func New() *App {
	return &App{}
}

func (a *App) Init() *App {
	_, callerDir, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Error generating env dir")
	}

	// Define the possible paths to the .env file
	envPaths := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "envs/.env"),
	}

	// Load the .env file from the provided paths
	env.LoadEnv(envPaths...) // Use ... to expand the slice
	config.NewConfig()

	loggerPath := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "tmp/logs"),
	}
	logger.NewLogger(loggerPath...)

	return a
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	container := iContainer.IContainer{}
	ic, infraDown, err := container.IContext(ctx).
		ICDown().ICPostgres().ICGrpc().ICEcho().NewIC()
	if err != nil {
		return err
	}
	defer infraDown()

	extBridge, extBridgeDown, err := externalBridge.NewExternalBridge(ctx)
	if err != nil {
		return err
	}
	defer extBridgeDown()

	me := configureModule(ctx, ic, extBridge)
	if me != nil {
		return me
	}

	var serverError error
	go func() {
		if err := ic.GrpcServer.RunGrpcServer(ctx, nil); err != nil {
			ic.Logger.Sugar().Errorf("(s.RunGrpcServer) err: {%v}", err)
			serverError = err
			cancel()
		}
	}()

	go func() {
		if err := ic.EchoHttpServer.RunServer(ctx, nil); err != nil {
			ic.Logger.Sugar().Errorf("(s.RunEchoServer) err: {%v}", err)
			serverError = err
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case v := <-quit:
		ic.Logger.Sugar().Errorf("signal.Notify: %v", v)
	case done := <-ctx.Done():
		ic.Logger.Sugar().Errorf("ctx.Done: %v", done)
	}

	ic.Logger.Sugar().Info("Server Exited Properly")
	return serverError
}

func configureModule(ctx context.Context, ic *iContainer.IContainer, extBridge *externalBridge.ExternalBridge) error {
	err := articleConfigurator.NewConfigurator(ic, extBridge).Configure(ctx)
	if err != nil {
		return err
	}

	err = healthCheckConfigurator.NewConfigurator(ic).Configure(ctx)
	if err != nil {
		return err
	}

	err = oauthConfigurator.NewConfigurator(ic, extBridge).Configure(ctx)
	if err != nil {
		return err
	}

	err = authConfigurator.NewConfigurator(ic, extBridge).Configure(ctx)
	if err != nil {
		return err
	}

	return nil
}


// cmd/load_data.go
package cmd

import (
	"context"
	"fmt"
	"github.com/RichardKnop/go-fixtures"
	"github.com/diki-haryadi/ztools/config"
	"github.com/diki-haryadi/ztools/env"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/postgres"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"log"
	"path/filepath"
	"runtime"
	"time"
)

var (
	loadDataCmd = &cobra.Command{
		Use:              "load_data",
		Short:            "Load data into the system",
		Long:             "Load data into the system for initialization or testing purposes",
		PersistentPreRun: loadDataPreRun,
		RunE:             runLoadData,
	}
)

func LoadDataCmd() *cobra.Command {
	return loadDataCmd
}

func loadDataPreRun(cmd *cobra.Command, args []string) {
	_, callerDir, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Error generating env dir")
	}

	// Define the possible paths to the .env file
	envPaths := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "envs/.env"),
	}

	// Load the .env file from the provided paths
	env.LoadEnv(envPaths...) // Use ... to expand the slice
	config.NewConfig()

	loggerPath := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "tmp/logs"),
	}
	logger.NewLogger(loggerPath...)

}

func runLoadData(cmd *cobra.Command, args []string) error {
	pg, err := postgres.NewConnection(context.Background(), &postgres.Config{
		Host:    config.BaseConfig.Postgres.Host,
		Port:    config.BaseConfig.Postgres.Port,
		User:    config.BaseConfig.Postgres.User,
		Pass:    config.BaseConfig.Postgres.Pass,
		DBName:  config.BaseConfig.Postgres.DBName,
		SslMode: config.BaseConfig.Postgres.SslMode,
	})
	defer pg.SqlxDB.Close()
	if err != nil {
		return err
	}

	_, callerDir, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Error generating env dir")
	}
	envPaths := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "db/fixtures/roles.yml"),
		filepath.Join(filepath.Dir(callerDir), "..", "db/fixtures/scopes.yml"),
		filepath.Join(filepath.Dir(callerDir), "..", "db/fixtures/test_access_tokens.yml"),
		filepath.Join(filepath.Dir(callerDir), "..", "db/fixtures/test_clients.yml"),
		filepath.Join(filepath.Dir(callerDir), "..", "db/fixtures/test_users.yml"),
	}

	bar := progressbar.NewOptions(len(envPaths),
		progressbar.OptionSetDescription("Loading fixtures..."),
		progressbar.OptionShowCount(),
		progressbar.OptionShowDescriptionAtLineEnd(),
	)

	for _, path := range envPaths {
		description := fmt.Sprintf("Processing: %s", filepath.Base(path))
		bar.Describe(description)
		err = fixtures.LoadFiles(envPaths, pg.SqlxDB.DB, "postgres")
		if err != nil {
			return err
		}

		bar.Add(1)
		time.Sleep(40 * time.Millisecond)
	}
	bar.Describe("Finished")
	fmt.Println("\nFinished loading data")
	return nil
}


// cmd/root.go
package cmd

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "main",
	Short: `
	 _______  _______             _______ _________ _______  _______  _______ 
	(  ____ \(  ___  )           (       )\__   __/(  ____ \(  ____ )(  ___  )
	| (    \/| (   ) |           | () () |   ) (   | (    \/| (    )|| (   ) |
	| |      | |   | |   _____   | || || |   | |   | |      | (____)|| |   | |
	| | ____ | |   | |  (_____)  | |(_)| |   | |   | |      |     __)| |   | |
	| | \_  )| |   | |           | |   | |   | |   | |      | (\ (   | |   | |
	| (___) || (___) |           | )   ( |___) (___| (____/\| ) \ \__| (___) |
	(_______)(_______)           |/     \|\_______/(_______/|/   \__/(_______) by Diki Haryadi
    `,
}

func Execute() {
	logrus.SetFormatter(&logrus.JSONFormatter{})

	rootCmd.AddCommand(ServeCmd())
	//ServeCmd().PersistentFlags().StringVarP(nil, "config", "c", "", "Config URL i.e. file://config.json")
	ServeCmd().Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.AddCommand(LoadDataCmd())
	//LoadDataCmd().PersistentFlags().StringVarP(nil, "config", "c", "", "Config URL i.e. file://config.json")
	LoadDataCmd().Flags().BoolP("toggle", "t", false, "Help message for toggle")

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalln("Error: \n", err.Error())
		os.Exit(1)
	}
}


// cmd/serve.go
package cmd

import (
	"fmt"
	"github.com/diki-haryadi/go-micro-template/app"
	"github.com/diki-haryadi/go-micro-template/config"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/spf13/cobra"
)

var (
	serveCmd = &cobra.Command{
		Use:              "serve",
		Short:            "A API for publish audio to kafka",
		Long:             "A API for publish audio to kafka",
		PersistentPreRun: servePreRun,
		RunE:             runServe,
	}
)

func ServeCmd() *cobra.Command {
	return serveCmd
}

func servePreRun(cmd *cobra.Command, args []string) {
	config.LoadConfig()
}

func runServe(cmd *cobra.Command, args []string) error {
	err := app.New().Init().Run()
	if err != nil {
		fmt.Println(err)
		logger.Zap.Sugar().Fatal(err)
	}

	return nil
}


// config/config.go
package config

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"log"
	"path/filepath"
	"runtime"
)

type Config struct {
	App AppConfig
}

var BaseConfig *Config

type AppConfig struct {
	AppEnv      string `json:"app_env" envconfig:"APP_ENV"`
	AppName     string `json:"app_name" envconfig:"APP_NAME"`
	ConfigOauth ConfigOauth
}

// Config stores all configuration options
type ConfigOauth struct {
	Oauth         OauthConfig
	Session       SessionConfig
	IsDevelopment bool
	JWTSecret     string `json:"jwt_secret" envconfig:"JWT_SECRET"`
}

// OauthConfig stores oauth service configuration options
type OauthConfig struct {
	AccessTokenLifetime  int `json:"access_token_lifetime" envconfig:"OAUTH_ACCESS_TOKEN_LIFETIME"`
	RefreshTokenLifetime int `json:"refresh_token_lifetime" envconfig:"OAUTH_REFRESH_TOKEN_LIFETIME"`
	AuthCodeLifetime     int `json:"auth_code_lifetime" envconfig:"OAUTH_AUTH_CODE_LIFETIME"`
}

// SessionConfig stores session configuration for the web app
type SessionConfig struct {
	Secret string `json:"secret" envconfig:"SESSION_SECRET"`
	Path   string `json:"path" envconfig:"SESSION_PATH"`
	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'.
	// MaxAge>0 means Max-Age attribute present and given in seconds.
	MaxAge int `json:"max_age" envconfig:"SESSION_MAX_AGE"`
	// When you tag a cookie with the HttpOnly flag, it tells the browser that
	// this particular cookie should only be accessed by the server.
	// Any attempt to access the cookie from client script is strictly forbidden.
	HTTPOnly bool `json:"http_only" envconfig:"SESSION_HTTP_ONLY"`
}

func LoadConfig() *Config {
	_, callerDir, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Error generating env dir")
	}

	// Define the possible paths to the .env file
	envPaths := []string{
		filepath.Join(filepath.Dir(callerDir), "..", "envs/.env"),
	}
	_ = godotenv.Overload(envPaths[0])
	var configLoader Config

	if err := envconfig.Process("BaseConfig", &configLoader); err != nil {
		log.Printf("error load config: %v", err)
	}

	BaseConfig = &configLoader
	spew.Dump(configLoader)
	return &configLoader
}


// db/fixtures/roles.yml
#-------#
# Roles #
#-------#

- table: 'roles'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440000'  # Example UUID for superuser
  fields:
    name: 'Superuser'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'roles'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440001'  # Example UUID for user
  fields:
    name: 'User'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'


// db/fixtures/scopes.yml
#--------#
# Scopes #
#--------#

- table: 'scopes'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440002'  # Example UUID for read scope
  fields:
    scope: 'read'
    description: 'Allows read access'  # Added description field
    is_default: true
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'scopes'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440003'  # Example UUID for read_write scope
  fields:
    scope: 'read_write'
    description: 'Allows both read and write access'  # Added description field
    is_default: false
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'
#--------#
# Scopes #
#--------#

- table: 'scopes'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440002'  # Example UUID for read scope
  fields:
    scope: 'read'
    description: 'Allows read access'  # Added description field
    is_default: true
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'scopes'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440003'  # Example UUID for read_write scope
  fields:
    scope: 'read_write'
    description: 'Allows both read and write access'  # Added description field
    is_default: false
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'


// db/fixtures/test_access_tokens.yml
#---------------#
# Access Tokens #
#---------------#

- table: 'access_tokens'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440005'  # Example UUID for the second token
  fields:
    client_id: '550e8400-e29b-41d4-a716-446655440008'  # Matching UUID from clients
    user_id: '550e8400-e29b-41d4-a716-44665544000a'    # Matching UUID from users
    token: 'test_superuser_token'
    scope: 'read read-write'
    expires_at: '2099-01-08 04:05:06'

- table: 'access_tokens'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440006'  # Example UUID for the third token
  fields:
    client_id: '550e8400-e29b-41d4-a716-446655440008'  # Matching UUID from clients
    user_id: '550e8400-e29b-41d4-a716-44665544000b'    # Matching UUID from users
    token: 'test_user_token'
    scope: 'read read-write'
    expires_at: '2099-01-08 04:05:06'

- table: 'access_tokens'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440007'  # Example UUID for the fourth token
  fields:
    client_id: '550e8400-e29b-41d4-a716-446655440008'  # Matching UUID from clients
    user_id: '550e8400-e29b-41d4-a716-44665544000c'    # Matching UUID from users
    token: 'test_user_token_2'
    scope: 'read read-write'
    expires_at: '2099-01-08 04:05:06'


// db/fixtures/test_clients.yml
#---------#
# Clients #
#---------#

- table: 'clients'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440008'  # Example UUID for the first client
  fields:
    key: 'test_client_1'
    secret: '$2a$10$CUoGytf1pR7CC6Y043gt/.vFJUV4IRqvH5R6F0VfITP8s2TqrQ.4e'
    redirect_uri: 'https://www.example.com'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'clients'
  pk:
    id: '550e8400-e29b-41d4-a716-446655440009'  # Example UUID for the second client
  fields:
    key: 'test_client_2'
    secret: '$2a$10$CUoGytf1pR7CC6Y043gt/.vFJUV4IRqvH5R6F0VfITP8s2TqrQ.4e'
    redirect_uri: 'https://www.example.com'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'


// db/fixtures/test_users.yml
#-------#
# Users #
#-------#

- table: 'users'
  pk:
    id: '550e8400-e29b-41d4-a716-44665544000a'  # Example UUID for the first user
  fields:
    role_id: '550e8400-e29b-41d4-a716-446655440000'  # Matching UUID for superuser role
    username: 'test@superuser'
    password: '$2a$10$FCcDkpgLjHVsJOltVHM9qey32W.zpYpVW3T0RIPVspsL8eUluhCSy' # test_password
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'users'
  pk:
    id: '550e8400-e29b-41d4-a716-44665544000b'  # Example UUID for the second user
  fields:
    role_id: '550e8400-e29b-41d4-a716-446655440003'  # Matching UUID for user role
    username: 'test@user'
    password: '$2a$10$FCcDkpgLjHVsJOltVHM9qey32W.zpYpVW3T0RIPVspsL8eUluhCSy'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'

- table: 'users'
  pk:
    id: '550e8400-e29b-41d4-a716-44665544000c'  # Example UUID for the third user
  fields:
    role_id: '550e8400-e29b-41d4-a716-446655440003'  # Matching UUID for user role
    username: 'test@user2'
    password: '$2a$10$FCcDkpgLjHVsJOltVHM9qey32W.zpYpVW3T0RIPVspsL8eUluhCSy'
    created_at: 'ON_INSERT_NOW()'
    updated_at: 'ON_UPDATE_NOW()'


// db/migrations/20221110221143_migrate_name.down.sql


// db/migrations/20221110221143_migrate_name.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";


CREATE TABLE articles (
  "id" uuid DEFAULT uuid_generate_v4 (),
  "name" text NOT NULL,
  "description" text NOT NULL
);

// db/migrations/20240908110637_users.down.sql
DROP TABLE users;


// db/migrations/20240908110637_users.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
   "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
   "username" VARCHAR(254) UNIQUE NOT NULL,
   "password" VARCHAR(60) NOT NULL,
   "role_id" VARCHAR(50) NOT NULL,
   "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
   "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
   "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
--  CONSTRAINT unique_username_role UNIQUE ("username", "role")  -- Optional: if a user can have only one role
);


// db/migrations/20241003072848_clients.down.sql
DROP TABLE clients;


// db/migrations/20241003072848_clients.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE clients (
       "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
       "key" VARCHAR(254) NOT NULL UNIQUE,  -- Renamed to avoid reserved keyword
       "secret" VARCHAR(128) NOT NULL,            -- Increased length for security
       "redirect_uri" VARCHAR(200) NOT NULL,      -- Consider adding a check constraint if needed
       "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
       "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
       "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
    );

-- Optional: Create an index on redirect_uri if you plan to query by it
CREATE INDEX idx_users_redirect_uri ON clients("redirect_uri");


// db/migrations/20241003072908_scopes.down.sql
DROP TABLE scopes;


// db/migrations/20241003072908_scopes.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE scopes (
    "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
    "scope" VARCHAR(200) NOT NULL UNIQUE,
    "description" VARCHAR(300) NULL,
    "is_default" VARCHAR(200) NOT NULL,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Optional: Create an index on 'description' if you plan to search by it frequently
CREATE INDEX idx_scopes_description ON scopes("description");


// db/migrations/20241003072922_roles.down.sql
DROP TABLE roles;


// db/migrations/20241003072922_roles.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE roles (
   "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
   "name" VARCHAR(200) NOT NULL UNIQUE,  -- Unique role name
   "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Timestamp of creation
   "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Timestamp of last update
   "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL  -- Timestamp of deletion
);

-- Optional: Create an index on 'name' if you plan to search by it frequently
CREATE INDEX idx_roles_name ON roles("name");


// db/migrations/20241003072940_refresh_tokens.down.sql
DROP TABLE refresh_token;


// db/migrations/20241003072940_refresh_tokens.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE refresh_tokens (
    "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
    "client_id" UUID NOT NULL,  -- Assuming client_id is a UUID
    "user_id" UUID NOT NULL,     -- Assuming user_id is a UUID
    "token" TEXT NOT NULL,  -- Increased length for token
    "expires_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL,  -- Expiration timestamp
    "scope" VARCHAR(50) NOT NULL,  -- Consider if this should be unique
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Indexes for performance
CREATE INDEX idx_refresh_tokens_client_id ON refresh_tokens("client_id");
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens("user_id");
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens("token");  -- Optional: index for token lookups


// db/migrations/20241003072953_access_tokens.down.sql
DROP TABLE access_tokens;


// db/migrations/20241003072953_access_tokens.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE access_tokens (
    "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
    "client_id" UUID NOT NULL,  -- Assuming client_id is a UUID
    "user_id" UUID NULL,     -- Assuming user_id is a UUID
    "token" TEXT NOT NULL,  -- Increased length for token
    "expires_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL,  -- Expiration timestamp
    "scope" VARCHAR(50) NOT NULL,  -- Consider if this should be unique
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Indexes for performance
CREATE INDEX idx_access_tokens_client_id ON access_tokens("client_id");
CREATE INDEX idx_access_tokens_user_id ON access_tokens("user_id");
CREATE INDEX idx_access_tokens_token ON access_tokens("token");  -- Optional: index for token lookups


// db/migrations/20241003073005_authorization_codes.down.sql
DROP TABLE authorization_codes;


// db/migrations/20241003073005_authorization_codes.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE authorization_codes (
   "id" uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
   "client_id" UUID NOT NULL,  -- Assuming client_id is a UUID
   "user_id" UUID NOT NULL,     -- Assuming user_id is a UUID
   "code" VARCHAR(300) NOT NULL,  -- Increased length for token
   "redirect_uri" VARCHAR(200) NOT NULL,
   "expires_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL,  -- Expiration timestamp
   "scope" VARCHAR(50) NOT NULL,  -- Consider if this should be unique
   "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
   "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
   "deleted_at" TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

-- Indexes for performance
CREATE INDEX idx_authorization_codes_client_id ON authorization_codes("client_id");
CREATE INDEX idx_authorization_codes_user_id ON authorization_codes("user_id");


// deployments/cassandra.yml
version: '3'  #choose version as per your need

services:
  cassandra:
    image: cassandra:latest
    container_name: cassandra-container
    ports:
      - "9042:9042"
    environment:
      - CASSANDRA_USER=admin
      - CASSANDRA_PASSWORD=admin
    volumes:
      - cassandra-data:/var/lib/cassandra

volumes:
  cassandra-data:

// deployments/docker-compose.e2e-local.yaml
version: "3.8"

volumes:
  test_pg_data:
  test_zookeeper_data:
    driver: local
  kafka_data:
    driver: local

services:
  postgres:
    container_name: pg_test_container
    image: postgres:11.16-alpine
    volumes:
      - "test_pg_data:/var/lib/postgresql/data"
    restart: always
    ports:
      - 5434:5432
    environment:
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: admin
      POSTGRES_DB: go-micro-template

  zookeeper:
    image: bitnami/zookeeper
    ports:
      - 12181:2181
    hostname: zookeeper
    environment:
      ALLOW_ANONYMOUS_LOGIN: "true"

  kafka:
    image: bitnami/kafka
    ports:
      - 9094:9092
    hostname: kafka
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_LISTENERS: "INTERNAL://:29092,EXTERNAL://:9092"
      KAFKA_ADVERTISED_LISTENERS: "INTERNAL://kafka:29092,EXTERNAL://localhost:9092"
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT"
      KAFKA_INTER_BROKER_LISTENER_NAME: "INTERNAL"
      ALLOW_PLAINTEXT_LISTENER: "yes"
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"
    #      KAFKA_CFG_LISTENERS: 'PLAINTEXT://:9092'
    #      KAFKA_CFG_ADVERTISED_LISTENERS: 'PLAINTEXT://:9092'
    #      KAFKA_CFG_ZOOKEEPER_CONNECT: zookeeper:2181
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - zookeeper

  kafdrop:
    image: obsidiandynamics/kafdrop
    restart: "no"
    ports:
      - 9000:9000
    environment:
      KAFKA_BROKERCONNECT: kafka:29092
      JVM_OPTS: "-Xms16M -Xmx48M -Xss180K -XX:-TieredCompilation -XX:+UseStringDeduplication -noverify"
    depends_on:
      - kafka


// deployments/docker-compose.yaml
version: '3.8'

volumes:
  pg_data:
  zookeeper_data:
    driver: local
  kafka_data:
    driver: local

services:
  postgres:
    container_name: pg_container
    image: postgres:11.16-alpine
    volumes:
      - 'pg_data:/var/lib/postgresql/data'
    restart: always
    ports:
      - 5432:5432
    environment:
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: admin
      POSTGRES_DB: go-micro-template

  zookeeper:
    image: bitnami/zookeeper
    ports:
      - 2181:2181
    hostname: zookeeper
    environment:
      ALLOW_ANONYMOUS_LOGIN: 'true'

  kafka:
    image: bitnami/kafka
    ports:
      - 9092:9092
    hostname: kafka
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_LISTENERS: "INTERNAL://:29092,EXTERNAL://:9092"
      KAFKA_ADVERTISED_LISTENERS: "INTERNAL://kafka:29092,EXTERNAL://localhost:9092"
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT"
      KAFKA_INTER_BROKER_LISTENER_NAME: "INTERNAL"
      ALLOW_PLAINTEXT_LISTENER: 'yes'
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: 'true'
#      KAFKA_CFG_LISTENERS: 'PLAINTEXT://:9092'
#      KAFKA_CFG_ADVERTISED_LISTENERS: 'PLAINTEXT://:9092'
#      KAFKA_CFG_ZOOKEEPER_CONNECT: zookeeper:2181
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    depends_on:
      - zookeeper

  kafdrop:
    image: obsidiandynamics/kafdrop
    restart: "no"
    ports:
      - 9000:9000
    environment:
      KAFKA_BROKERCONNECT: kafka:29092
      JVM_OPTS: "-Xms16M -Xmx48M -Xss180K -XX:-TieredCompilation -XX:+UseStringDeduplication -noverify"
    depends_on:
      - kafka


// docs/admin.http
### Manajemen Client (Client Management)
## Register Client Baru
POST /api/v1/oauth/clients
Content-Type: application/json
{
    "name": "My Application",
    "redirect_uris": ["https://app.example.com/callback"],
    "grant_types": ["authorization_code", "refresh_token"],
    "scope": "read write",
    "confidential": true
}

### Mendapatkan Daftar Client
GET /api/v1/oauth/clients

### Update Client
PUT /api/v1/oauth/clients/{client_id}

### Hapus Client
DELETE /api/v1/oauth/clients/{client_id}

### Manajemen User (User Management)
### Register User Baru
POST /api/v1/users
{
    "username": "john.doe@example.com",
    "password": "secure_password",
    "name": "John Doe",
    "roles": ["user"]
}

### Update Profile User
PUT /api/v1/users/profile

### Reset Password
POST /api/v1/users/reset-password

### Verifikasi Email
POST /api/v1/users/verify-email

### Manajemen User (Admin)
GET /api/v1/users
###
PUT /api/v1/users/{user_id}
###
DELETE /api/v1/users/{user_id}
###

### Manajemen Scope (Scope Management)
# Buat Scope Baru
POST /api/v1/oauth/scopes
{
    "name": "read_profile",
    "description": "Read user profile information"
}

### Daftar Scope
GET /api/v1/oauth/scopes

### Update Scope
PUT /api/v1/oauth/scopes/{scope_id}

### Hapus Scope
DELETE /api/v1/oauth/scopes/{scope_id}

### Manajemen Token (Token Management)
# Daftar Active Tokens
GET /api/v1/oauth/tokens

### Revoke Token
POST /api/v1/oauth/tokens/revoke
{
    "token": "access_token_value",
    "token_type_hint": "access_token"
}

### Bulk Revoke Tokens (untuk user tertentu atau client)
POST /api/v1/oauth/tokens/bulk-revoke
{
    "user_id": "user_id",
    "client_id": "client_id"
}

### Consent Management
# User Consent untuk Client
POST /api/v1/oauth/consents
{
    "client_id": "client_id",
    "scopes": ["read", "write"]
}

### Daftar User Consents
GET /api/v1/oauth/consents

### Hapus Consent
DELETE /api/v1/oauth/consents/{consent_id}

#### Security & Monitoring
# Audit Log
GET /api/v1/audit-logs

### Rate Limiting Status
GET /api/v1/oauth/rate-limits

### Active Sessions
GET /api/v1/oauth/sessions
###
DELETE /api/v1/oauth/sessions/{session_id}

#Database Schema yang diperlukan:

#users - Menyimpan informasi user
#clients - Menyimpan informasi OAuth clients
#access_tokens - Menyimpan access tokens
#refresh_tokens - Menyimpan refresh tokens
#authorization_codes - Menyimpan authorization codes
#scopes - Menyimpan available scopes
#client_scopes - Relasi many-to-many client dan scopes
#user_consents - Menyimpan user consents untuk clients
#audit_logs - Menyimpan audit trail

#Fitur Keamanan yang perlu diimplementasikan:
#
#Rate Limiting
#Token Encryption
#Password Hashing
#IP Whitelisting
#CORS Configuration
#Request Validation
#Audit Logging
#Session Management

// docs/api-specification/authorization_code.md
{
"data": {
"user_id": "550e8400-e29b-41d4-a716-44665544000a",
"access_token": "dd5d2260-b429-43df-9be8-0965e5702201",
"expires_in": 3600,
"token_type": "Bearer",
"scope": "read_write",
"refresh_token": "f1fd5b88-e5ee-4ee2-91ba-df557006def1"
},
"message": "Success",
"code": "STATUS_OK",
"status_code": 200,
"status": "success"
}

// docs/api-specification/introspect.md
___

### API Endpoint

**POST** `/v1/oauth/introspect`

___

### Request Headers

-   **Authorization**: `Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==`
    -   Description: Base64-encoded client credentials for basic authentication.
-   **Content-Type**: `application/json`
    -   Description: Indicates the request payload is in JSON format.

___

### Request Body

The request body must be in JSON format and contain the following fields:

```
{
  "token": "string",
  "token_type_hint": "string"
}
```

-   **token** (string, required): The OAuth 2.0 token to be introspected.
-   **token\_type\_hint** (string, optional): A hint about the type of the token submitted for introspection (e.g., `access_token` or `refresh_token`).

___

### Response

The response will be in JSON format, providing details about the token's validity and associated information.

#### Successful Response

```
{
  "active": true,
  "scope": "read write",
  "client_id": "test_client_1",
  "username": "user123",
  "token_type": "Bearer",
  "exp": 1640995200,
  "iat": 1640918800,
  "nbf": 1640918800,
  "sub": "user123",
  "aud": "my_api",
  "iss": "http://localhost:4000",
  "jti": "b8e3bc09-cf9a-40c5-a5ad-e5f1240c2f4a"
}

```

-   **active** (boolean): Indicates if the token is currently active.
-   **scope** (string): The scopes associated with the token.
-   **client\_id** (string): The client identifier for the token.
-   **username** (string): The username associated with the token, if applicable.
-   **token\_type** (string): The type of the token (e.g., `Bearer`).
-   **exp** (integer): The token expiration time in Unix epoch format.
-   **iat** (integer): The time at which the token was issued in Unix epoch format.
-   **nbf** (integer): The time before which the token must not be accepted, in Unix epoch format.
-   **sub** (string): The subject of the token, usually a user ID or username.
-   **aud** (string): The intended audience of the token.
-   **iss** (string): The issuer of the token.
-   **jti** (string): A unique identifier for the token.

___

#### Error Response

-   **active** (boolean): Indicates the token is not active or invalid.

___

{
"code": "404000",
"status_code": 404,
"status": "undefined",
"error": "Access token not found"
}

// docs/api-specification/oauth_credentials.md

{
"data": {
"access_token": "241b49a6-bb2e-4af6-ad75-9495e182789c",
"expires_in": 3600,
"token_type": "Bearer",
"scope": "read_write"
},
"message": "Success",
"code": "STATUS_OK",
"status_code": 200,
"status": "success"
}

// docs/api-specification/password.md
{
"data": {
"user_id": "550e8400-e29b-41d4-a716-44665544000b",
"access_token": "6dedb757-c573-4d05-8435-0b0a5cf33829",
"expires_in": 3600,
"token_type": "Bearer",
"scope": "read_write",
"refresh_token": "b5f601cd-ad17-4e06-a8fc-e2eee234e8ff"
},
"message": "Success",
"code": "STATUS_OK",
"status_code": 200,
"status": "success"
}

// docs/api-specification/refresh_token.md
{
"message": "Refresh token not found",
"code": "404000",
"status_code": 404,
"status": "not found",
"error": "Refresh token not found"
}


// docs/common-oauth2-server-feature.md
# API Feature Common
1. oauth2/revoke
2. /oauth2/userinfo
3. Register Endpoint (/register):
```
POST /register
{
"email": "user@example.com",
"password": "password123",
"name": "John Doe",
"additional_fields": {}
}

Response:
{
"user_id": "12345",
"message": "Registration successful"
}
```

3. Password Reset Flow:
```
// Request reset
POST /forgot-password
{
"email": "user@example.com"
}

// Reset with token
POST /reset-password
{
"token": "reset_token",
"new_password": "newpass123"
}
```
4. Profile Management:
```
// Get profile
GET /profile
Authorization: Bearer <access_token>

// Update profile
PUT /profile
Authorization: Bearer <access_token>
{
  "name": "Updated Name",
  "additional_fields": {}
}
```

// docs/db.md
# Database

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Migrations](#migrations)
  - [Installation](#installation)
  - [Usage](#usage)
- [PostgreSQL](#postgresql)
  - [Installation](#installation-1)
  - [Connecting to the Database](#connecting-to-the-database)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

This project uses PostgreSQL as its primary data store, and Migrate for managing database migrations.

## Migrations

This code repository use [Migrate](https://github.com/golang-migrate/migrate) as a database migration tool. It helps you manage and keep track of database schema changes in your Go applications.

### Installation

To install migrate cli please go to [installation guide](https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md).

### Usage

You can use the following command to set the PG_URL environment variable and then use it in the migrate command:

```bash
export PG_URL=postgres://[user]:[password]@[host]:[port]/[database]
```

This can be useful if you need to run the command multiple times and don't want to type out the full database URL each time.

To create a new migration, use the following command:

```makefile
migrate-create: # Target for running 'create'
	migrate create -ext sql -dir db/migrations $(NAME)
```

This will create a new migration file in the db/migrations directory with the name [timestamp]_[migration_name].up.sql and [timestamp]_[migration_name].down.sql, which contain the SQL statements for applying and rolling back the migration, respectively.

To run the migrations, use the following command:

```makefile
migrate-up:   # Target for running 'up' command
 migrate -path db/migrations -database  $(PG_URL) up
```

To rollback the last migration, use the following command:

```makefile
migrate-down: # Target for running 'down' command
	migrate -path db/migrations -database $(PG_URL) down
```

To drop all tables and sequences in the database, use the following command:

```makefile
migrate-drop: # Target for running 'drop' command
	migrate drop -database $(PG_URL)
```

**Note**: This command will permanently delete all data in the database, so use caution when running it.

To apply or rollback a migration to a specific version , use the following command:

```makefile
migrate-force: # Target for running 'force' command
 migrate -path db/migrations -database $(PG_URL) force $(VERSION)
```

This is useful when you want to undo or redo a specific migration, or when you want to apply a migration that was previously rolled back.
**Note**: The migrate force command should be used with caution, as it can permanently alter the state of the database. Make sure you have a backup of your data before using this command.

## PostgreSQL

PostgreSQL is a powerful, open-source object-relational database system with a strong reputation for reliability, feature robustness, and performance. It is commonly used as the primary data store for web, mobile, geospatial, and analytics applications.

Some key features of PostgreSQL include:

- Support for multiple data types, including text, numerical, and spatial data
- Support for ACID transactions, which ensure the consistency and integrity of data
- Support for triggers, stored procedures, and views, which allow you to define custom logic and data manipulation operations
- Support for full-text search and advanced indexing options, which enable fast querying and data retrieval
- Support for JSON and JSONB data types, which allow you to store and manipulate complex, nested data structures

### Installation

To get started with PostgreSQL, you will need to install the database server and client libraries on your machine. There are various ways to install PostgreSQL, including using a package manager, downloading the binaries from the official website or using an existing Docker Compose file.

Follow these steps:

1. Make sure that Docker and Docker Compose are installed on your machine.
2. Open the docker-compose.yml file in your project directory.
3. If the PostgreSQL service definition already exists in Docker Compose file, you can start the PostgreSQL container by running the following command:

```bash
docker-compose up -d
```

This will start the PostgreSQL container in detached mode.

To stop the container, use the following command:

```bash
docker-compose stop
```

### Connecting to the Database

Replace myuser, mypassword, and mydatabase with the desired username, password, and database name in the docker-coompose file.

To connect to the PostgreSQL database from your application, use the following connection string:

```
postgres://myuser:mypassword
```

Once you have installed PostgreSQL, you can create a database and start using it in your application by connecting to it using a database driver, such as the pgx driver for Go.

To learn more about PostgreSQL, you can refer to the official documentation at https://www.postgresql.org/docs/.


// docs/deployment.md
# Deployment

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Prerequisites](#prerequisites)
- [Setup](#setup)
- [Verification](#verification)
- [Maintenance](#maintenance)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

This guide will walk you through the steps to deploy the Golang microservice application using Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)

## Setup

1. Clone the repository and navigate to the root directory of the project:

```bash
git clone https://github.com/infranyx/go-microservice-template
cd go-microservice-template
```

2. Create a .env file in the `envs` directory of the project and copy the `local.env` environment variables.

3. Run the following command to build and start the containers:

```bash
docker-compose up -d --build
```

This will build and start the following containers:

- `postgres`: PostgreSQL database
- `kafka`: Kafka message broker
- `zookeeper`: In the context of Kafka, Zookeeper is used to store metadata about the Kafka cluster and its topics. It helps the Kafka brokers maintain their cluster membership and elect leaders, and it also helps clients discover the location of the Kafka brokers.
- `redis`: Redis cache
- `sentry`: Sentry error tracking service
- `app`: Golang microservice application

## Verification

To verify that the containers are running, use the following command:

```bash
docker-compose ps
```

## Maintenance

To stop the containers, use the following command:

```bash
docker-compose stop
```

To start the containers again, use the following command:

```bash
docker-compose start
```

To remove the containers, use the following command:

```bash
docker-compose down
```


// docs/design.md
# Design

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Key Components and Features](#key-components-and-features)
- [Design Decisions](#design-decisions)
  - [See also](#see-also)
- [Protocol Buffer](#protocol-buffer)
- [API docs](#api-docs)
- [Layout](#layout)
- [Error Handling](#error-handling)
- [Diagrams and Mockups](#diagrams-and-mockups)
- [Open Issues and Areas for Improvement](#open-issues-and-areas-for-improvement)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

This document outlines the design of the project, including the key components and features, design decisions, and technologies used.

## Key Components and Features

The project includes the following components and features:

- A PostgreSQL database for storing and querying data
- A Kafka message broker for reliable, scalable event streaming
- A Redis cache for improving performance and reducing load on the database
- A gRPC API for efficient communication between services
- An Echo framework for building HTTP APIs and web applications
- Sentry integration for error tracking and reporting

## Design Decisions

The following design decisions were made:

- PostgreSQL was chosen as the primary data store because of its robust support for ACID transactions and ability to handle large volumes of data.
- Kafka was chosen as the message broker because of its high performance and ability to handle large amounts of data.
- Redis was chosen as the cache because of its in-memory storage and ability to support multiple data structures.
- gRPC was chosen as the API technology because of its efficient binary encoding and ability to support streaming requests and responses.
- Echo was chosen as the web framework because of its lightweight and easy-to-use design.
- Sentry was chosen for error tracking and reporting because of its comprehensive feature set and integrations with a wide range of technologies.
- The project follows a clean architecture design pattern, with separate layers for the domain logic, application logic, and infrastructure. - This helps to improve the maintainability and testability of the codebase.

### See also

- [gRPC](https://grpc.io/) for communication between services
- [SQLx](https://github.com/jmoiron/sqlx) for database access and migrations
- [Redis](github.com/go-redis/redis) for caching and message queues
- [Kafka](https://github.com/segmentio/kafka-go) for streaming data processing
- [Echo](https://echo.labstack.com/) for web server routing
- [Zap Logger](https://github.com/uber-go/zap) for logging
- [Sentry](https://sentry.io/) for error tracking and reporting
- [Cron](https://godoc.org/github.com/robfig/cron) for scheduling tasks
- [errors](https://github.com/pkg/errors) for error handling and adding stack trace to golang
- [OZZO](github.com/go-ozzo/ozzo-validation) for data validation

## Protocol Buffer

To use protocol buffer for gRPC communication please refer to [Protohub](https://github.com/infranyx/protobuf-template). Protohub is a hub for managing your protobuf files and with auto generation feature you can simply `go get` the generated code of your proto.

## API docs

The template doesn't have API docs. For auto-generated API docs that you include, you can also give instructions on the
build process.

## Layout

The project is organized into a number of directories and subdirectories, as shown in the following tree structure:

```tree
├── .github
│  ├── CODEOWNERS
│  └── pull_request_template.md
├── app
│  └── app.go
├── cmd
│  └── main.go
├── db
│  └── migrations
│    └── time_migrate_name.up.sql
├── envs
│  ├── .env
│  ├── local.env
│  ├── production.env
│  ├── stage.env
│  └── test.env
├── external
│  └── service_name_module
│    ├── domain
│    ├── exception
│    ├── dto
│    └── usecase
├── internal
│  └── module_name
│    ├── configurator
│    ├── delivery
│    │  ├── grpc
│    │  ├── http
│    │  └── kafka
│    │    ├── consumer
│    │    └── producer
│    ├── domain
│    ├── dto
│    ├── exception
│    ├── job
│    ├── repository
│    ├── tests
│    │  ├── fixtures
│    │  └── integrations
│    └── usecase
├── pkg
│  ├── client_container
│  ├── config
│  ├── constant
│  ├── cron
│  ├── env
│  ├── error
│  │  ├── contracts
│  │  ├── custom_error
│  │  ├── error_utils
│  │  ├── grpc
│  │  └── http
│  ├── grpc
│  ├── http
│  ├── infra_container
│  ├── kafka
│  │  ├── consumer
│  │  └── producer
│  ├── logger
│  ├── postgres
│  ├── redis
│  ├── sentry
│  └── wrapper
│
├── .gitignore
├── .pre-commit-config.yaml
├── golangci.yaml
├── docker-compose.e2e-local.yaml
├── docker-compose.yaml
├── go.sum
├── Makefile
├── go.mod
├── LICENSE
└── README.md
```

- `.github`: Contains GitHub-specific files, such as CODEOWNERS and pull request template.
- `app`: Contains the entry point of the application.
- `cmd:` Contains the main command of the application.
- `db`: Contains the database migrations.
- `envs`: Contains the environment configuration files.
- `external`: Contains the external service modules.
- `internal`: Contains the internal modules.
- `pkg`: Contains the shared packages.
- `.gitignore`: Defines which files and directories should be ignored by Git.
- `.pre-commit-config.yaml`: Defines the pre-commit hooks and checks.
- `golangci.yaml`: Defines the configuration for the GolangCI linter.
- `docker-compose.yaml`: Defines the configuration for the Docker Compose containers.
- `go.sum`: Contains the checksums for the Go module dependencies.
- `Makefile`: Contains the build and development tasks.
- `go.mod`: Defines the Go module dependencies.
- `LICENSE`: Contains the license terms for the project.
- `README.md`: Contains the project documentation.

## Error Handling

The project includes a custom error implementation to handle errors in a consistent and structured way. The custom error implementation includes a set of predefined error types for different categories of errors, such as application errors, bad request errors, conflict errors, and so on. This allows us to classify and handle errors in a standardized way, making it easier to understand and debug issues that may arise.

The custom error implementation is located in the error package in the pkg directory. It includes the following error types:

- `ApplicationError`: Represents an error that occurs within the application logic, such as a failure to process a request or a failure to access a required resource.
- `BadRequestError`: Represents an error that occurs when the request is invalid or malformed, such as a missing or invalid parameter.
- `ConflictError`: Represents an error that occurs when the request cannot be completed due to a conflict with the current state of the resource, such as a duplicate key error.
- `DomainError`: Represents an error that occurs within the domain logic, such as a failure to meet a business rule or a failure to access a required domain resource.
- `ForbiddenError`: Represents an error that occurs when the request is forbidden, such as when the user does not have the necessary permissions to access the resource.
- `InternalError`: Represents an error that occurs within the infrastructure layer, such as a failure to connect to a database or a failure to access a required service.
- `MarshalingError`: Represents an error that occurs when marshaling or unmarshaling data, such as when converting between different data formats.
- `NotFoundError`: Represents an error that occurs when the requested resource is not found.
- `UnauthorizedError`: Represents an error that occurs when the request is unauthorized, such as when the user is not authenticated or does not have the necessary permissions to access the resource.
- `ValidationError`: Represents an error that occurs when the request data is invalid, such as when a required field is missing or a field is in the wrong format.

The custom error implementation also includes utility functions for creating and wrapping errors, as well as functions for handling and responding to errors in different contexts, such as when handling HTTP requests or when working with gRPC.

To use the custom error implementation, the application code can import the error package and use the error types and utility functions as needed. For example, to create a new BadRequestError, the code can use the NewBadRequestError function:

```golang
import "project/pkg/error/custum_errors"

err := customErrors.NewBadRequestError("invalid parameter", code)
```

To wrap an existing error with a custom error type, the code can use the WrapError function:

```golang
import "project/pkg/error/custum_errors"

err := customErrors.NewBadRequestErrorWrap(err, "invalid parameter", code)
```

## Diagrams and Mockups

To include diagrams or mockups in the design.md file to illustrate the design of your project, you will need to create these visualizations using a diagramming tool or software. Here are a few options for creating diagrams and mockups:

- dbdiagram.io: an online tool for creating and sharing database diagrams. It allows you to design the schema of your database visually, using a simple drag-and-drop interface. You can add tables, columns, and relationships to the diagram, and customize the appearance of the elements using a range of formatting options.
- Draw.io: A web-based diagramming tool that allows you to create a wide range of diagrams, including flowcharts, mind maps, and UML diagrams.
- Lucidchart: A web-based diagramming tool that offers a range of templates and shapes for creating professional-quality diagrams.
- Figma: A web-based design and prototyping tool that allows you to create wireframes, mockups, and prototypes for web and mobile applications.

Once you have created the diagrams or mockups that you want to include in the design.md file, you can add them to the file by including the images inline or by linking to the images.

[Include any relevant diagrams or mockups to illustrate the design]

## Open Issues and Areas for Improvement

[Describe any open issues or areas for improvement in the design]


// docs/devops.md
# DevOps Documentations

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

<!-- END doctoc generated TOC please keep comment here to allow auto update -->


// docs/env.md
# Environment Variables

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

This file lists the environment variables that are used by the project.

| Variable                         | Description                                                             | Required | Default Value    |
| -------------------------------- | ----------------------------------------------------------------------- | -------- | ---------------- |
| `APP_ENV`                        | The environment that the application is running in                      | Yes      | `prod`           |
| `GRPC_HOST`                      | The hostname for the gRPC server                                        | Yes      | `localhost`      |
| `GRPC_PORT`                      | The port for the gRPC server                                            | Yes      | `3000`           |
| `HTTP_PORT`                      | The port for the HTTP server                                            | Yes      | N/A              |
| `EXTERNAL_GO_TEMPLATE_GRPC_PORT` | The port for an external gRPC server                                    | Yes      | `3000`           |
| `EXTERNAL_GO_TEMPLATE_GRPC_HOST` | The hostname for an external gRPC server                                | Yes      | `localhost`      |
| `PG_HOST`                        | The hostname for the Postgresql server                                  | Yes      | `localhost`      |
| `PG_PORT`                        | The port for the Postgresql server                                      | Yes      | `5432`           |
| `PG_USER`                        | The username for the Postgresql server                                  | Yes      | `admin`          |
| `PG_PASS`                        | The password for the Postgresql server                                  | Yes      | `admin`          |
| `PG_DB`                          | The database name for the Postgresql server                             | Yes      | `grpc_template`  |
| `PG_MAX_CONNECTIONS`             | The maximum number of connections allowed to the Postgresql server      | Yes      | `1`              |
| `PG_MAX_IDLE_CONNECTIONS`        | The maximum number of idle connections allowed to the Postgresql server | Yes      | `1`              |
| `PG_MAX_LIFETIME_CONNECTIONS`    | The maximum lifetime of connections to the Postgresql server            | Yes      | `1`              |
| `PG_SSL_MODE`                    | The SSL mode for the Postgresql server                                  | Yes      | `disable`        |
| `KAFKA_ENABLED`                  | Whether or not Kafka is enabled                                         | Yes      | `1`              |
| `KAFKA_LOG_EVENTS`               | Whether or not to log events to Kafka                                   | Yes      | `1`              |
| `KAFKA_CLIENT_ID`                | The client ID for Kafka                                                 | Yes      | `dev-consumer`   |
| `KAFKA_CLIENT_GROUP_ID`          | The client group ID for Kafka                                           | Yes      | `dev-group`      |
| `KAFKA_CLIENT_BROKERS`           | The brokers for Kafka                                                   | Yes      | `localhost:9094` |
| `KAFKA_NAMESPACE`                | The namespace for Kafka                                                 | Yes      | `dev`            |
| `KAFKA_TOPIC`                    | The topic for Kafka                                                     | Yes      | `test-topic`     |
| `SENTRY_DSN`                     | The Data Source Name (DSN) for Sentry                                   | Yes      | `*`              |


// docs/flow-diagram.md
sequenceDiagram
participant C as Client/Frontend
participant AS as Authorization Server
participant RS as Resource Server

    %% Client Credentials Flow
    rect rgb(200, 220, 240)
    Note over C,RS: Client Credentials Flow
    C->>AS: POST /oauth/tokens
    Note right of C: grant_type=client_credentials<br/>Basic Auth: client_id:client_secret<br/>scope=read_write
    AS->>C: Returns access_token
    C->>RS: API request with access_token
    RS->>AS: POST /oauth/introspect<br/>Validate token
    AS->>RS: Token validation response
    RS->>C: Protected resource response
    end

    %% Password Flow
    rect rgb(220, 240, 200)
    Note over C,RS: Password Flow
    C->>AS: POST /oauth/tokens
    Note right of C: grant_type=password<br/>Basic Auth: client_id:client_secret<br/>username & password<br/>scope=read_write
    AS->>C: Returns access_token + refresh_token
    C->>RS: API request with access_token
    RS->>AS: POST /oauth/introspect<br/>Validate token
    AS->>RS: Token validation response
    RS->>C: Protected resource response
    end

    %% Authorization Code Flow
    rect rgb(240, 220, 200)
    Note over C,RS: Authorization Code Flow
    C->>AS: GET /oauth/authorize
    Note right of C: response_type=code<br/>client_id<br/>redirect_uri<br/>scope
    AS->>C: Redirect with auth code
    C->>AS: POST /oauth/tokens
    Note right of C: grant_type=authorization_code<br/>code<br/>redirect_uri<br/>Basic Auth: client_id:client_secret
    AS->>C: Returns access_token + refresh_token
    C->>RS: API request with access_token
    RS->>AS: POST /oauth/introspect<br/>Validate token
    AS->>RS: Token validation response
    RS->>C: Protected resource response
    end

    %% Refresh Token Flow
    rect rgb(240, 200, 220)
    Note over C,RS: Refresh Token Flow
    C->>AS: POST /oauth/tokens
    Note right of C: grant_type=refresh_token<br/>refresh_token<br/>Basic Auth: client_id:client_secret
    AS->>C: Returns new access_token + refresh_token
    C->>RS: API request with new access_token
    RS->>AS: POST /oauth/introspect<br/>Validate token
    AS->>RS: Token validation response
    RS->>C: Protected resource response
    end

sequenceDiagram
participant Admin
participant Client
participant User
participant AS as Authorization Server

    %% Client Management
    rect rgb(200, 220, 240)
    Note over Admin,AS: Client Management APIs
    Admin->>AS: POST /api/v1/oauth/clients
    Note right of Admin: Register new client<br/>name, redirect_uris, grant_types
    AS->>Admin: Returns client_id & client_secret

    Admin->>AS: GET /api/v1/oauth/clients
    Note right of Admin: List all registered clients

    Admin->>AS: PUT /api/v1/oauth/clients/{client_id}
    Note right of Admin: Update client details

    Admin->>AS: DELETE /api/v1/oauth/clients/{client_id}
    Note right of Admin: Remove client registration
    end

    %% User Management
    rect rgb(220, 240, 200)
    Note over Admin,AS: User Management APIs
    Admin->>AS: POST /api/v1/users
    Note right of Admin: Register new user<br/>username, password, roles

    Admin->>AS: GET /api/v1/users
    Note right of Admin: List all users

    User->>AS: PUT /api/v1/users/profile
    Note right of User: Update user profile

    Admin->>AS: DELETE /api/v1/users/{user_id}
    Note right of Admin: Deactivate user
    end

    %% Scope Management
    rect rgb(240, 220, 200)
    Note over Admin,AS: Scope Management APIs
    Admin->>AS: POST /api/v1/oauth/scopes
    Note right of Admin: Create new scope<br/>name, description

    Admin->>AS: GET /api/v1/oauth/scopes
    Note right of Admin: List all available scopes

    Admin->>AS: PUT /api/v1/oauth/scopes/{scope_id}
    Note right of Admin: Update scope details
    end

    %% Token Management
    rect rgb(240, 200, 220)
    Note over Admin,AS: Token Management APIs
    Admin->>AS: GET /api/v1/oauth/tokens
    Note right of Admin: List active tokens

    Admin->>AS: DELETE /api/v1/oauth/tokens/{token_id}
    Note right of Admin: Revoke specific token

    Client->>AS: POST /api/v1/oauth/tokens/revoke
    Note right of Client: Revoke token by value
    end

// docs/oauth.http
### Oauth Introspect
POST http://localhost:4000/api/v1/oauth/introspect
Authorization: Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==
Content-Type: application/x-www-form-urlencoded

token = 00ccd40e-72ca-4e79-a4b6-67c95e2e3f1c &
token_type_hint = access_token

### Oauth Refresh Tokens
POST http://localhost:4000/api/v1/oauth/tokens
Authorization: Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==
Content-Type: application/x-www-form-urlencoded

grant_type = refresh_token &
refresh_token = 6fd8d272-375a-4d8a-8d0f-43367dc8b791

### Oauth Client Credentials
POST http://localhost:4000/api/v1/oauth/tokens
Authorization: Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==
Content-Type: application/x-www-form-urlencoded

grant_type = client_credentials &
scope = read_write

### Oauth Password
###
POST http://localhost:4000/api/v1/oauth/tokens
Authorization: Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==
Content-Type: application/x-www-form-urlencoded

grant_type = password &
username = test@user &
password = test_password &
scope = read_write

### Oauth Authorization Code
POST http://localhost:4000/api/v1/oauth/tokens
Authorization: Basic dGVzdF9jbGllbnRfMTp0ZXN0X3NlY3JldA==
Content-Type: application/x-www-form-urlencoded

grant_type = authorization_code &
code = 7afb1c55-76e4-4c76-adb7-9d657cb47a27 &
redirect_uri = https://www.example.com

###

// docs/requirement/user-consents.md
User consents (persetujuan pengguna) adalah komponen penting dalam OAuth2 yang berfungsi untuk mencatat dan mengelola izin yang diberikan pengguna kepada aplikasi client. Mari saya jelaskan lebih detail:
OAuth2 Consent Flow and ManagementClick to open diagram
Fungsi utama menyimpan user consents:

Transparansi dan Kontrol

Mencatat secara eksplisit izin apa saja yang diberikan user kepada aplikasi
User bisa melihat aplikasi mana saja yang memiliki akses ke akunnya
User bisa mencabut akses aplikasi kapan saja


Automated Authorization

Menghindari user harus menyetujui ulang setiap kali aplikasi yang sama meminta akses
Mempercepat proses autentikasi untuk aplikasi yang sudah disetujui sebelumnya


Granular Permission Control

Menyimpan scope spesifik yang diizinkan untuk setiap aplikasi
Memungkinkan user memberikan akses terbatas (misal: read-only)
Memungkinkan update izin tanpa harus mencabut semua akses



Contoh struktur tabel user_consents:
```aiignore
TABLE user_consents (
id UUID PRIMARY KEY,
user_id UUID NOT NULL,
client_id UUID NOT NULL,
scopes TEXT[] NOT NULL,
created_at TIMESTAMP NOT NULL,
last_used_at TIMESTAMP,
expires_at TIMESTAMP,
FOREIGN KEY (user_id) REFERENCES users(id),
FOREIGN KEY (client_id) REFERENCES clients(id)
);  
```

Use cases penting:

Security Audit

Melacak kapan dan kepada siapa user memberikan akses
Membantu dalam investigasi jika terjadi masalah keamanan
Memastikan compliance dengan regulasi privacy


User Experience
```curl
// Contoh response GET /oauth/consents
{
  "consents": [
    {
      "client_name": "Spotify App",
      "scopes": ["read_profile", "read_playlist"],
      "last_used": "2024-11-26T10:00:00Z",
      "granted_at": "2024-10-01T14:30:00Z"
    }
  ]
}
```
Revocation Management

Saat user mencabut akses:

Hapus consent record
Revoke semua active tokens
Batalkan semua refresh tokens
Kirim notifikasi ke client (optional)




Compliance & Privacy

Membantu memenuhi requirement GDPR tentang consent
Menyediakan audit trail untuk privacy compliance
Memungkinkan user mengexport data consent mereka



Best practices implementasi:

Selalu timestamp setiap consent record
Implement soft delete untuk history audit
Set up automatic cleanup untuk expired consents
Provide clear UI untuk user memanage consents
Implement webhook notifications untuk consent changes

Apakah Anda ingin saya menjelaskan lebih detail tentang implementasi spesifik atau aspek tertentu dari consent management?

```sequenceDiagram
participant U as User
participant C as Client App
participant AS as Auth Server
participant DB as Consent DB

    rect rgb(200, 220, 240)
        Note over U,DB: First Time Authorization Flow
        C->>AS: Request Authorization
        AS->>DB: Check Existing Consent
        DB->>AS: No Consent Found
        AS->>U: Display Consent Screen
        Note over U,AS: Shows requested permissions:<br/>- Read Profile<br/>- Access Email<br/>- etc
        U->>AS: Approve Permissions
        AS->>DB: Store Consent Record
        AS->>C: Grant Authorization
    end

    rect rgb(220, 240, 200)
        Note over U,DB: Subsequent Authorization
        C->>AS: Request Authorization
        AS->>DB: Check Existing Consent
        DB->>AS: Consent Found
        AS->>C: Auto-grant Authorization
        Note over C,AS: Skip consent screen<br/>if permissions match
    end

    rect rgb(240, 220, 200)
        Note over U,DB: Consent Management
        U->>AS: View Authorized Apps
        AS->>DB: Fetch User's Consents
        DB->>AS: Return Consent List
        AS->>U: Display Authorized Apps
        U->>AS: Revoke App Access
        AS->>DB: Delete Consent Record
        AS->>DB: Revoke Related Tokens
    end```




// envs/local.env
# Application
APP_ENV=local
OAUTH_ACCESS_TOKEN_LIFETIME=3600
OAUTH_REFRESH_TOKEN_LIFETIME=1209600
OAUTH_AUTH_CODE_LIFETIME=3600
JWT_SECRET=secret
#AccessTokenLifetime:  3600,    // 1 hour
#RefreshTokenLifetime: 1209600, // 14 days
#AuthCodeLifetime:     3600,    // 1 hour

# GRPC
GRPC_HOST=localhost
GRPC_PORT=3000

# HTTP
HTTP_HOST=localhost
HTTP_PORT=4000

# Sample External GRPC client
SAMPLE_EXT_SERVICE_GRPC_HOST=localhost
SAMPLE_EXT_SERVICE_GRPC_PORT=3000

# Postgresql
PG_HOST=localhost
PG_PORT=5432
PG_USER=admin
PG_PASS=admin
PG_DB=go-micro-template
PG_MAX_CONNECTIONS=1
PG_MAX_IDLE_CONNECTIONS=1
PG_MAX_LIFETIME_CONNECTIONS=1
PG_SSL_MODE=disable

# Kafka
KAFKA_ENABLED=1
KAFKA_LOG_EVENTS=1
KAFKA_CLIENT_ID=local-consumer
KAFKA_CLIENT_GROUP_ID=local-group
KAFKA_CLIENT_BROKERS=localhost:9092
KAFKA_NAMESPACE=local
KAFKA_TOPIC=test-topic

# Sentry
SENTRY_DSN=*

// envs/production.env
# Application
APP_ENV=prod
OAUTH_ACCESS_TOKEN_LIFETIME=3600
OAUTH_REFRESH_TOKEN_LIFETIME=1209600
OAUTH_AUTH_CODE_LIFETIME=3600
JWT_SECRET=secret
#AccessTokenLifetime:  3600,    // 1 hour
#RefreshTokenLifetime: 1209600, // 14 days
#AuthCodeLifetime:     3600,    // 1 hour

# GRPC
GRPC_HOST=localhost
GRPC_PORT=3000

# HTTP
HTTP_HOST=localhost
HTTP_PORT=4000

# Sample External GRPC client
SAMPLE_EXT_SERVICE_GRPC_HOST=localhost
SAMPLE_EXT_SERVICE_GRPC_PORT=3000

# Postgresql
PG_HOST=localhost
PG_PORT=5432
PG_USER=admin
PG_PASS=admin
PG_DB=go-micro-template
PG_MAX_CONNECTIONS=1
PG_MAX_IDLE_CONNECTIONS=1
PG_MAX_LIFETIME_CONNECTIONS=1
PG_SSL_MODE=disable

# Kafka
KAFKA_ENABLED=1
KAFKA_LOG_EVENTS=1
KAFKA_CLIENT_ID=prod-consumer
KAFKA_CLIENT_GROUP_ID=prod-group
KAFKA_CLIENT_BROKERS=localhost:9092
KAFKA_NAMESPACE=prod
KAFKA_TOPIC=test-topic

# Sentry
SENTRY_DSN=*

// envs/stage.env
# Application
APP_ENV = stage
OAUTH_ACCESS_TOKEN_LIFETIME=3600
OAUTH_REFRESH_TOKEN_LIFETIME=1209600
OAUTH_AUTH_CODE_LIFETIME=3600
JWT_SECRET=secret
#AccessTokenLifetime:  3600,    // 1 hour
#RefreshTokenLifetime: 1209600, // 14 days
#AuthCodeLifetime:     3600,    // 1 hour

# GRPC
GRPC_HOST=localhost
GRPC_PORT=3000

# HTTP
HTTP_HOST=localhost
HTTP_PORT=4000

# Sample External GRPC client
SAMPLE_EXT_SERVICE_GRPC_HOST=localhost
SAMPLE_EXT_SERVICE_GRPC_PORT=3000

# Postgresql
PG_HOST=localhost
PG_PORT=5432
PG_USER=admin
PG_PASS=admin
PG_DB=go-micro-template
PG_MAX_CONNECTIONS=1
PG_MAX_IDLE_CONNECTIONS=1
PG_MAX_LIFETIME_CONNECTIONS=1
PG_SSL_MODE=disable

# Kafka
KAFKA_ENABLED=1
KAFKA_LOG_EVENTS=1
KAFKA_CLIENT_ID=stage-consumer
KAFKA_CLIENT_GROUP_ID=stage-group
KAFKA_CLIENT_BROKERS=localhost:9092
KAFKA_NAMESPACE=stage
KAFKA_TOPIC=test-topic

# Sentry
SENTRY_DSN=*

// envs/test.env
# Application
APP_ENV=test
OAUTH_ACCESS_TOKEN_LIFETIME=3600
OAUTH_REFRESH_TOKEN_LIFETIME=1209600
OAUTH_AUTH_CODE_LIFETIME=3600
JWT_SECRET=secret
#AccessTokenLifetime:  3600,    // 1 hour
#RefreshTokenLifetime: 1209600, // 14 days
#AuthCodeLifetime:     3600,    // 1 hour

# GRPC
GRPC_HOST=localhost
GRPC_PORT=3000

# HTTP
HTTP_HOST=localhost
HTTP_PORT=4000

# Sample External GRPC client
SAMPLE_EXT_SERVICE_GRPC_HOST=localhost
SAMPLE_EXT_SERVICE_GRPC_PORT=3000

# Postgresql
PG_HOST=localhost
PG_PORT=5432
PG_USER=admin
PG_PASS=admin
PG_DB=go-micro-template
PG_MAX_CONNECTIONS=1
PG_MAX_IDLE_CONNECTIONS=1
PG_MAX_LIFETIME_CONNECTIONS=1
PG_SSL_MODE=disable

# Kafka
KAFKA_ENABLED=1
KAFKA_LOG_EVENTS=1
KAFKA_CLIENT_ID=test-consumer
KAFKA_CLIENT_GROUP_ID=test-group
KAFKA_CLIENT_BROKERS=localhost:9092
KAFKA_NAMESPACE=test
KAFKA_TOPIC=test-topic

# Sentry
SENTRY_DSN=*

// external/sample_ext_service/domain/sample_ext_service_domain.go
package sampleExtServiceDomain

import (
	"context"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
)

type SampleExtServiceUseCase interface {
	CreateArticle(ctx context.Context, req *articleV1.CreateArticleRequest) (*articleV1.CreateArticleResponse, error)
}


// external/sample_ext_service/usecase/sample_ext_service_usecase.go
package sampleExtServiceUseCase

import (
	"context"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"

	sampleExtServiceDomain "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/domain"
	grpcError "github.com/diki-haryadi/ztools/error/grpc"
	"github.com/diki-haryadi/ztools/grpc"
)

type sampleExtServiceUseCase struct {
	grpcClient grpc.Client
}

func NewSampleExtServiceUseCase(grpcClient grpc.Client) sampleExtServiceDomain.SampleExtServiceUseCase {
	return &sampleExtServiceUseCase{
		grpcClient: grpcClient,
	}
}

func (esu *sampleExtServiceUseCase) CreateArticle(ctx context.Context, req *articleV1.CreateArticleRequest) (*articleV1.CreateArticleResponse, error) {
	articleGrpcClient := articleV1.NewArticleServiceClient(esu.grpcClient.GetGrpcConnection())

	res, err := articleGrpcClient.CreateArticle(ctx, req)
	if err != nil {
		return nil, grpcError.ParseExternalGrpcErr(err)
	}

	return res, nil
}


// go.mod
module github.com/diki-haryadi/go-micro-template

go 1.23.4

require (
	github.com/RichardKnop/go-fixtures v0.0.0-20191226211317-8d7ddb76c9e2
	github.com/davecgh/go-spew v1.1.1
	github.com/diki-haryadi/protobuf-ecomerce v0.1.9
	github.com/diki-haryadi/protobuf-template v0.0.0-20241114145947-cffb40e44840
	github.com/diki-haryadi/ztools v0.0.14
	github.com/go-ozzo/ozzo-validation v3.6.0+incompatible
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/sessions v1.4.0
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/labstack/echo/v4 v4.13.0
	github.com/lib/pq v1.10.9
	github.com/pkg/errors v0.9.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/schollz/progressbar/v3 v3.17.1
	github.com/segmentio/kafka-go v0.4.47
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.30.0
	golang.org/x/sys v0.28.0
	google.golang.org/grpc v1.69.2
)

require (
	github.com/getsentry/sentry-go v0.29.1 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241104194629-dd2ea8efbc28 // indirect
	google.golang.org/protobuf v1.36.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)


// go.sum
cloud.google.com/go v0.26.0/go.mod h1:aQUYkXzVsufM+DwF1aE+0xfcU+56JwCaLick0ClmMTw=
filippo.io/edwards25519 v1.1.0 h1:FNf4tywRC1HmFuKW5xopWpigGjJKiJSV0Cqo0cJWDaA=
filippo.io/edwards25519 v1.1.0/go.mod h1:BxyFTGdWcka3PhytdK4V28tE5sGfRvvvRV7EaN4VDT4=
github.com/BurntSushi/toml v0.3.1/go.mod h1:xHWCNGjB5oqiDr8zfno3MHue2Ht5sIBksp03qcyfWMU=
github.com/RichardKnop/go-fixtures v0.0.0-20191226211317-8d7ddb76c9e2 h1:V0U1q5+5AEmwnijzGmveBG1luark7fNrW3yKdFkuwX0=
github.com/RichardKnop/go-fixtures v0.0.0-20191226211317-8d7ddb76c9e2/go.mod h1:AkkGIv1ar1WtAmNGXaqm3YeK4qXD/GhrqhaLctZIjQE=
github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 h1:DklsrG3dyBCFEj5IhUbnKptjxatkF07cF2ak3yi77so=
github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2/go.mod h1:WaHUgvxTVq04UNunO+XhnAqY/wQc+bxr74GqbsZ/Jqw=
github.com/benbjohnson/clock v1.1.0/go.mod h1:J11/hYXuz8f4ySSvYwY0FKfm+ezbsZBKZxNJlLklBHA=
github.com/census-instrumentation/opencensus-proto v0.2.1/go.mod h1:f6KPmirojxKA12rnyqOA5BBL4O983OfeGPqjHWSTneU=
github.com/chengxilo/virtualterm v1.0.4 h1:Z6IpERbRVlfB8WkOmtbHiDbBANU7cimRIof7mk9/PwM=
github.com/chengxilo/virtualterm v1.0.4/go.mod h1:DyxxBZz/x1iqJjFxTFcr6/x+jSpqN0iwWCOK1q10rlY=
github.com/client9/misspell v0.3.4/go.mod h1:qj6jICC3Q7zFZvVWo7KLAzC3yx5G7kyvSDkc90ppPyw=
github.com/cncf/udpa/go v0.0.0-20191209042840-269d4d468f6f/go.mod h1:M8M6+tZqaGXZJjfX53e64911xZQV5JYwmTeXPW+k8Sc=
github.com/cpuguy83/go-md2man/v2 v2.0.4/go.mod h1:tgQtvFlXSQOSOSIRvRPT7W67SCa46tRHOmNcaadrF8o=
github.com/davecgh/go-spew v1.1.0/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/diki-haryadi/protobuf-ecomerce v0.1.9 h1:wWGvCmnisRLyvEBrrL3lsglKwT1zBUGk8O2KcRXxamA=
github.com/diki-haryadi/protobuf-ecomerce v0.1.9/go.mod h1:MtqkMTq0DLX9Q+oD9czJPheaFrhg4zqq3Jbo1oHzxQ0=
github.com/diki-haryadi/protobuf-template v0.0.0-20241114145947-cffb40e44840 h1:2GsALvTfXFcq6O5HlEYtjB9xYA55J7iq2vgkjYFj2SY=
github.com/diki-haryadi/protobuf-template v0.0.0-20241114145947-cffb40e44840/go.mod h1:gIrRM9XyrN6zDsy51I8NuGOLvUd5r+D+b1d8snB2VZI=
github.com/diki-haryadi/ztools v0.0.14 h1:cahZg1Lygo4iEwadIaaFAmP/6psYMSlSQDpoGauiNzs=
github.com/diki-haryadi/ztools v0.0.14/go.mod h1:hBT0DlPWPtsmz//+sxpqS3kPKEYKYxJDPT/uHZaEzIY=
github.com/envoyproxy/go-control-plane v0.9.0/go.mod h1:YTl/9mNaCwkRvm6d1a2C3ymFceY/DCBVvsKhRF0iEA4=
github.com/envoyproxy/go-control-plane v0.9.1-0.20191026205805-5f8ba28d4473/go.mod h1:YTl/9mNaCwkRvm6d1a2C3ymFceY/DCBVvsKhRF0iEA4=
github.com/envoyproxy/go-control-plane v0.9.4/go.mod h1:6rpuAdCZL397s3pYoYcLgu1mIlRU8Am5FuJP05cCM98=
github.com/envoyproxy/protoc-gen-validate v0.1.0/go.mod h1:iSmxcyjqTsJpI2R4NaDN7+kN2VEUnK/pcBlmesArF7c=
github.com/getsentry/sentry-go v0.29.1 h1:DyZuChN8Hz3ARxGVV8ePaNXh1dQ7d76AiB117xcREwA=
github.com/getsentry/sentry-go v0.29.1/go.mod h1:x3AtIzN01d6SiWkderzaH28Tm0lgkafpJ5Bm3li39O0=
github.com/go-errors/errors v1.4.2 h1:J6MZopCL4uSllY1OfXM374weqZFFItUbrImctkmUxIA=
github.com/go-errors/errors v1.4.2/go.mod h1:sIVyrIiJhuEF+Pj9Ebtd6P/rEYROXFi3BopGUQ5a5Og=
github.com/go-kit/log v0.1.0/go.mod h1:zbhenjAZHb184qTLMA9ZjW7ThYL0H2mk7Q6pNt4vbaY=
github.com/go-logfmt/logfmt v0.5.0/go.mod h1:wCYkCAKZfumFQihp8CzCvQ3paCTfi41vtzG1KdI/P7A=
github.com/go-logr/logr v1.4.2 h1:6pFjapn8bFcIbiKo3XT4j/BhANplGihG6tvd+8rYgrY=
github.com/go-logr/logr v1.4.2/go.mod h1:9T104GzyrTigFIr8wt5mBrctHMim0Nb2HLGrmQ40KvY=
github.com/go-logr/stdr v1.2.2 h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=
github.com/go-logr/stdr v1.2.2/go.mod h1:mMo/vtBO5dYbehREoey6XUKy/eSumjCCveDpRre4VKE=
github.com/go-ozzo/ozzo-validation v3.6.0+incompatible h1:msy24VGS42fKO9K1vLz82/GeYW1cILu7Nuuj1N3BBkE=
github.com/go-ozzo/ozzo-validation v3.6.0+incompatible/go.mod h1:gsEKFIVnabGBt6mXmxK0MoFy+cZoTJY6mu5Ll3LVLBU=
github.com/go-sql-driver/mysql v1.8.1 h1:LedoTUt/eveggdHS9qUFC1EFSa8bU2+1pZjSRpvNJ1Y=
github.com/go-sql-driver/mysql v1.8.1/go.mod h1:wEBSXgmK//2ZFJyE+qWnIsVGmvmEKlqwuVSjsCm7DZg=
github.com/go-stack/stack v1.8.0/go.mod h1:v0f6uXyyMGvRgIKkXu+yp6POWl0qKG85gN/melR3HDY=
github.com/gogo/protobuf v1.3.2 h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=
github.com/gogo/protobuf v1.3.2/go.mod h1:P1XiOD3dCwIKUDQYPy72D8LYyHL2YPYrpS2s69NZV8Q=
github.com/golang-jwt/jwt/v5 v5.2.1 h1:OuVbFODueb089Lh128TAcimifWaLhJwVflnrgM17wHk=
github.com/golang-jwt/jwt/v5 v5.2.1/go.mod h1:pqrtFR0X4osieyHYxtmOUWsAWrfe1Q5UVIyoH402zdk=
github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b/go.mod h1:SBH7ygxi8pfUlaOkMMuAQtPIUF8ecWP5IEl/CR7VP2Q=
github.com/golang/mock v1.1.1/go.mod h1:oTYuIxOrZwtPieC+H1uAHpcLFnEyAGVDL/k47Jfbm0A=
github.com/golang/protobuf v1.2.0/go.mod h1:6lQm79b+lXiMfvg/cZm0SGofjICqVBUtrP5yJMmIC1U=
github.com/golang/protobuf v1.3.2/go.mod h1:6lQm79b+lXiMfvg/cZm0SGofjICqVBUtrP5yJMmIC1U=
github.com/golang/protobuf v1.3.3/go.mod h1:vzj43D7+SQXF/4pzW/hwtAqwc6iTitCiVSaWz5lYuqw=
github.com/golang/protobuf v1.5.4 h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=
github.com/golang/protobuf v1.5.4/go.mod h1:lnTiLA8Wa4RWRcIUkrtSVa5nRhsEGBg48fD6rSs7xps=
github.com/google/go-cmp v0.2.0/go.mod h1:oXzfMopK8JAjlY9xF4vHSVASa0yLyX7SntLO5aqRK0M=
github.com/google/go-cmp v0.6.0 h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI=
github.com/google/go-cmp v0.6.0/go.mod h1:17dUlkBOakJ0+DkrSSNjCkIjxS6bF9zb3elmeNGIjoY=
github.com/google/gofuzz v1.2.0 h1:xRy4A+RhZaiKjJ1bPfwQ8sedCA+YS2YcCHW6ec7JMi0=
github.com/google/gofuzz v1.2.0/go.mod h1:dBl0BpW6vV/+mYPU4Po3pmUjxk6FQPldtuIdl/M65Eg=
github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=
github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
github.com/gorilla/securecookie v1.1.2 h1:YCIWL56dvtr73r6715mJs5ZvhtnY73hBvEF8kXD8ePA=
github.com/gorilla/securecookie v1.1.2/go.mod h1:NfCASbcHqRSY+3a8tlWJwsQap2VX5pwzwo4h3eOamfo=
github.com/gorilla/sessions v1.4.0 h1:kpIYOp/oi6MG/p5PgxApU8srsSw9tuFbt46Lt7auzqQ=
github.com/gorilla/sessions v1.4.0/go.mod h1:FLWm50oby91+hl7p/wRxDth9bWSuk0qVL2emc7lT5ik=
github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 h1:UH//fgunKIs4JdUbpDl1VZCDaL56wXCB/5+wF6uHfaI=
github.com/grpc-ecosystem/go-grpc-middleware v1.4.0/go.mod h1:g5qyo/la0ALbONm6Vbp88Yd8NsDy6rZz+RcrMPxvld8=
github.com/inconshreveable/mousetrap v1.1.0 h1:wN+x4NVGpMsO7ErUn/mUI3vEoE6Jt13X2s0bqwp9tc8=
github.com/inconshreveable/mousetrap v1.1.0/go.mod h1:vpF70FUmC8bwa3OWnCshd2FqLfsEA9PFc4w1p2J65bw=
github.com/jmoiron/sqlx v1.4.0 h1:1PLqN7S1UYp5t4SrVVnt4nUVNemrDAtxlulVe+Qgm3o=
github.com/jmoiron/sqlx v1.4.0/go.mod h1:ZrZ7UsYB/weZdl2Bxg6jCRO9c3YHl8r3ahlKmRT4JLY=
github.com/joho/godotenv v1.5.1 h1:7eLL/+HRGLY0ldzfGMeQkb7vMd0as4CfYvUVzLqw0N0=
github.com/joho/godotenv v1.5.1/go.mod h1:f4LDr5Voq0i2e/R5DDNOoa2zzDfwtkZa6DnEwAbqwq4=
github.com/kelseyhightower/envconfig v1.4.0 h1:Im6hONhd3pLkfDFsbRgu68RDNkGF1r3dvMUtDTo2cv8=
github.com/kelseyhightower/envconfig v1.4.0/go.mod h1:cccZRl6mQpaq41TPp5QxidR+Sa3axMbJDNb//FQX6Gg=
github.com/kisielk/errcheck v1.5.0/go.mod h1:pFxgyoBC7bSaBwPgfKdkLd5X25qrDl4LWUI2bnpBCr8=
github.com/kisielk/gotool v1.0.0/go.mod h1:XhKaO+MFFWcvkIS/tQcRk01m1F5IRFswLeQ+oQHNcck=
github.com/klauspost/compress v1.15.9/go.mod h1:PhcZ0MbTNciWF3rruxRgKxI5NkcHHrHUDtV4Yw2GlzU=
github.com/klauspost/compress v1.17.7 h1:ehO88t2UGzQK66LMdE8tibEd1ErmzZjNEqWkjLAKQQg=
github.com/klauspost/compress v1.17.7/go.mod h1:Di0epgTjJY877eYKx5yC51cX2A2Vl2ibi7bDH9ttBbw=
github.com/konsorten/go-windows-terminal-sequences v1.0.1/go.mod h1:T0+1ngSBFLxvqU3pZ+m/2kptfBszLMUkC4ZK/EgS/cQ=
github.com/kr/pretty v0.1.0 h1:L/CwN0zerZDmRFUapSPitk6f+Q3+0za1rQkzVuMiMFI=
github.com/kr/pretty v0.1.0/go.mod h1:dAy3ld7l9f0ibDNOQOHHMYYIIbhfbHSm3C4ZsoJORNo=
github.com/kr/pty v1.1.1/go.mod h1:pFQYn66WHrOpPYNljwOMqo10TkYh1fy3cYio2l3bCsQ=
github.com/kr/text v0.1.0/go.mod h1:4Jbv+DJW3UT/LiOwJeYQe1efqtUx/iVham/4vfdArNI=
github.com/kr/text v0.2.0 h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=
github.com/kr/text v0.2.0/go.mod h1:eLer722TekiGuMkidMxC/pM04lWEeraHUUmBw8l2grE=
github.com/labstack/echo/v4 v4.13.0 h1:8DjSi4H/k+RqoOmwXkxW14A2H1pdPdS95+qmdJ4q1Tg=
github.com/labstack/echo/v4 v4.13.0/go.mod h1:61j7WN2+bp8V21qerqRs4yVlVTGyOagMBpF0vE7VcmM=
github.com/labstack/gommon v0.4.2 h1:F8qTUNXgG1+6WQmqoUWnz8WiEU60mXVVw0P4ht1WRA0=
github.com/labstack/gommon v0.4.2/go.mod h1:QlUFxVM+SNXhDL/Z7YhocGIBYOiwB0mXm1+1bAPHPyU=
github.com/lib/pq v1.0.0/go.mod h1:5WUZQaWbwv1U+lTReE5YruASi9Al49XbQIvNi/34Woo=
github.com/lib/pq v1.10.9 h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS4JEWq7dtCCRUEw=
github.com/lib/pq v1.10.9/go.mod h1:AlVN5x4E4T544tWzH6hKfbfQvm3HdbOxrmggDNAPY9o=
github.com/mattn/go-colorable v0.1.13 h1:fFA4WZxdEF4tXPZVKMLwD8oUnCTTo08duU7wxecdEvA=
github.com/mattn/go-colorable v0.1.13/go.mod h1:7S9/ev0klgBDR4GtXTXX8a3vIGJpMovkB8vQcUbaXHg=
github.com/mattn/go-isatty v0.0.16/go.mod h1:kYGgaQfpe5nmfYZH+SKPsOc2e4SrIfOl2e/yFXSvRLM=
github.com/mattn/go-isatty v0.0.20 h1:xfD0iDuEKnDkl03q4limB+vH+GxLEtL/jb4xVJSWWEY=
github.com/mattn/go-isatty v0.0.20/go.mod h1:W+V8PltTTMOvKvAeJH7IuucS94S2C6jfK/D7dTCTo3Y=
github.com/mattn/go-runewidth v0.0.16 h1:E5ScNMtiwvlvB5paMFdw9p4kSQzbXFikJ5SQO6TULQc=
github.com/mattn/go-runewidth v0.0.16/go.mod h1:Jdepj2loyihRzMpdS35Xk/zdY8IAYHsh153qUoGf23w=
github.com/mattn/go-sqlite3 v1.10.0/go.mod h1:FPy6KqzDD04eiIsT53CuJW3U88zkxoIYsOqkbpncsNc=
github.com/mattn/go-sqlite3 v1.14.22 h1:2gZY6PC6kBnID23Tichd1K+Z0oS6nE/XwU+Vz/5o4kU=
github.com/mattn/go-sqlite3 v1.14.22/go.mod h1:Uh1q+B4BYcTPb+yiD3kU8Ct7aC0hY9fxUwlHK0RXw+Y=
github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db h1:62I3jR2EmQ4l5rM/4FEfDWcRD+abF5XlKShorW5LRoQ=
github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db/go.mod h1:l0dey0ia/Uv7NcFFVbCLtqEBQbrT4OCwCSKTEv6enCw=
github.com/opentracing/opentracing-go v1.1.0/go.mod h1:UkNAQd3GIcIGf0SeVgPpRdFStlNbqXla1AfSYxPUl2o=
github.com/pierrec/lz4/v4 v4.1.15 h1:MO0/ucJhngq7299dKLwIMtgTfbkoSPF6AoMYDd8Q4q0=
github.com/pierrec/lz4/v4 v4.1.15/go.mod h1:gZWDp/Ze/IJXGXf23ltt2EXimqmTUXEy0GFuRQyBid4=
github.com/pingcap/errors v0.11.4 h1:lFuQV/oaUMGcD2tqt+01ROSmJs75VG1ToEOkZIZ4nE4=
github.com/pingcap/errors v0.11.4/go.mod h1:Oi8TUi2kEtXXLMJk9l1cGmz20kV3TaQ0usTwv5KuLY8=
github.com/pkg/errors v0.8.1/go.mod h1:bwawxfHBFNV+L2hUp1rHADufV3IMtnDRdf1r5NINEl0=
github.com/pkg/errors v0.9.1 h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=
github.com/pkg/errors v0.9.1/go.mod h1:bwawxfHBFNV+L2hUp1rHADufV3IMtnDRdf1r5NINEl0=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4/go.mod h1:xMI15A0UPsDsEKsMN9yxemIoYk6Tm2C1GtYGdfGttqA=
github.com/rivo/uniseg v0.4.7 h1:WUdvkW8uEhrYfLC4ZzdpI2ztxP1I582+49Oc5Mq64VQ=
github.com/rivo/uniseg v0.4.7/go.mod h1:FN3SvrM+Zdj16jyLfmOkMNblXMcoc8DfTHruCPUcx88=
github.com/robfig/cron/v3 v3.0.1 h1:WdRxkvbJztn8LMz/QEvLN5sBU+xKpSqwwUO1Pjr4qDs=
github.com/robfig/cron/v3 v3.0.1/go.mod h1:eQICP3HwyT7UooqI/z+Ov+PtYAWygg1TEWWzGIFLtro=
github.com/russross/blackfriday/v2 v2.1.0/go.mod h1:+Rmxgy9KzJVeS9/2gXHxylqXiyQDYRxCVz55jmeOWTM=
github.com/schollz/progressbar/v3 v3.17.1 h1:bI1MTaoQO+v5kzklBjYNRQLoVpe0zbyRZNK6DFkVC5U=
github.com/schollz/progressbar/v3 v3.17.1/go.mod h1:RzqpnsPQNjUyIgdglUjRLgD7sVnxN1wpmBMV+UiEbL4=
github.com/segmentio/kafka-go v0.4.47 h1:IqziR4pA3vrZq7YdRxaT3w1/5fvIH5qpCwstUanQQB0=
github.com/segmentio/kafka-go v0.4.47/go.mod h1:HjF6XbOKh0Pjlkr5GVZxt6CsjjwnmhVOfURM5KMd8qg=
github.com/sirupsen/logrus v1.4.2/go.mod h1:tLMulIdttU9McNUspp0xgXVQah82FyeX6MwdIuYE2rE=
github.com/sirupsen/logrus v1.9.3 h1:dueUQJ1C2q9oE3F7wvmSGAaVtTmUizReu6fjN8uqzbQ=
github.com/sirupsen/logrus v1.9.3/go.mod h1:naHLuLoDiP4jHNo9R0sCBMtWGeIprob74mVsIT4qYEQ=
github.com/spf13/cobra v1.8.1 h1:e5/vxKd/rZsfSJMUX1agtjeTDf+qv1/JdBF8gg5k9ZM=
github.com/spf13/cobra v1.8.1/go.mod h1:wHxEcudfqmLYa8iTfL+OuZPbBZkmvliBWKIezN3kD9Y=
github.com/spf13/pflag v1.0.5 h1:iy+VFUOCP1a+8yFto/drg2CJ5u0yRoB7fZw3DKv/JXA=
github.com/spf13/pflag v1.0.5/go.mod h1:McXfInJRrz4CZXVZOBLb0bTZqETkiAhM9Iw0y3An2Bg=
github.com/stretchr/objx v0.1.0/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
github.com/stretchr/objx v0.1.1/go.mod h1:HFkY916IF+rwdDfMAkV7OtwuqBVzrE8GR6GFx+wExME=
github.com/stretchr/objx v0.4.0/go.mod h1:YvHI0jy2hoMjB+UWwv71VJQ9isScKT/TqJzVSSt89Yw=
github.com/stretchr/testify v1.2.2/go.mod h1:a8OnRcib4nhh0OaRAV+Yts87kKdq0PP7pXfy6kDkUVs=
github.com/stretchr/testify v1.3.0/go.mod h1:M5WIy9Dh21IEIfnGCwXGc5bZfKNJtfHm1UVUgZn+9EI=
github.com/stretchr/testify v1.4.0/go.mod h1:j7eGeouHqKxXV5pUuKE4zz7dFj8WfuZ+81PSLYec5m4=
github.com/stretchr/testify v1.7.0/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.7.1/go.mod h1:6Fq8oRcR53rry900zMqJjRRixrwX3KX962/h/Wwjteg=
github.com/stretchr/testify v1.8.0/go.mod h1:yNjHg4UonilssWZ8iaSj1OCr/vHnekPRkoO+kdMU+MU=
github.com/stretchr/testify v1.10.0 h1:Xv5erBjTwe/5IxqUQTdXv5kgmIvbHo3QQyRwhJsOfJA=
github.com/stretchr/testify v1.10.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
github.com/valyala/bytebufferpool v1.0.0 h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=
github.com/valyala/bytebufferpool v1.0.0/go.mod h1:6bBcMArwyJ5K/AmCkWv1jt77kVWyCJ6HpOuEn7z0Csc=
github.com/valyala/fasttemplate v1.2.2 h1:lxLXG0uE3Qnshl9QyaK6XJxMXlQZELvChBOCmQD0Loo=
github.com/valyala/fasttemplate v1.2.2/go.mod h1:KHLXt3tVN2HBp8eijSv/kGJopbvo7S+qRAEEKiv+SiQ=
github.com/xdg-go/pbkdf2 v1.0.0 h1:Su7DPu48wXMwC3bs7MCNG+z4FhcyEuz5dlvchbq0B0c=
github.com/xdg-go/pbkdf2 v1.0.0/go.mod h1:jrpuAogTd400dnrH08LKmI/xc1MbPOebTwRqcT5RDeI=
github.com/xdg-go/scram v1.1.2 h1:FHX5I5B4i4hKRVRBCFRxq1iQRej7WO3hhBuJf+UUySY=
github.com/xdg-go/scram v1.1.2/go.mod h1:RT/sEzTbU5y00aCK8UOx6R7YryM0iF1N2MOmC3kKLN4=
github.com/xdg-go/stringprep v1.0.4 h1:XLI/Ng3O1Atzq0oBs3TWm+5ZVgkq2aqdlvP9JtoZ6c8=
github.com/xdg-go/stringprep v1.0.4/go.mod h1:mPGuuIYwz7CmR2bT9j4GbQqutWS1zV24gijq1dTyGkM=
github.com/yuin/goldmark v1.1.27/go.mod h1:3hX8gzYuyVAZsxl0MRgGTJEmQBFcNTphYh9decYSb74=
github.com/yuin/goldmark v1.2.1/go.mod h1:3hX8gzYuyVAZsxl0MRgGTJEmQBFcNTphYh9decYSb74=
github.com/yuin/goldmark v1.4.13/go.mod h1:6yULJ656Px+3vBD8DxQVa3kxgyrAnzto9xy5taEt/CY=
go.opentelemetry.io/otel v1.31.0 h1:NsJcKPIW0D0H3NgzPDHmo0WW6SptzPdqg/L1zsIm2hY=
go.opentelemetry.io/otel v1.31.0/go.mod h1:O0C14Yl9FgkjqcCZAsE053C13OaddMYr/hz6clDkEJE=
go.opentelemetry.io/otel/metric v1.31.0 h1:FSErL0ATQAmYHUIzSezZibnyVlft1ybhy4ozRPcF2fE=
go.opentelemetry.io/otel/metric v1.31.0/go.mod h1:C3dEloVbLuYoX41KpmAhOqNriGbA+qqH6PQ5E5mUfnY=
go.opentelemetry.io/otel/sdk v1.31.0 h1:xLY3abVHYZ5HSfOg3l2E5LUj2Cwva5Y7yGxnSW9H5Gk=
go.opentelemetry.io/otel/sdk v1.31.0/go.mod h1:TfRbMdhvxIIr/B2N2LQW2S5v9m3gOQ/08KsbbO5BPT0=
go.opentelemetry.io/otel/sdk/metric v1.31.0 h1:i9hxxLJF/9kkvfHppyLL55aW7iIJz4JjxTeYusH7zMc=
go.opentelemetry.io/otel/sdk/metric v1.31.0/go.mod h1:CRInTMVvNhUKgSAMbKyTMxqOBC0zgyxzW55lZzX43Y8=
go.opentelemetry.io/otel/trace v1.31.0 h1:ffjsj1aRouKewfr85U2aGagJ46+MvodynlQ1HYdmJys=
go.opentelemetry.io/otel/trace v1.31.0/go.mod h1:TXZkRk7SM2ZQLtR6eoAWQFIHPvzQ06FJAsO1tJg480A=
go.uber.org/atomic v1.7.0/go.mod h1:fEN4uk6kAWBTFdckzkM89CLk9XfWZrxpCo0nPH17wJc=
go.uber.org/goleak v1.1.10/go.mod h1:8a7PlsEVH3e/a/GLqe5IIrQx6GzcnRmZEufDUTk4A7A=
go.uber.org/goleak v1.3.0 h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=
go.uber.org/goleak v1.3.0/go.mod h1:CoHD4mav9JJNrW/WLlf7HGZPjdw8EucARQHekz1X6bE=
go.uber.org/multierr v1.6.0/go.mod h1:cdWPpRnG4AhwMwsgIHip0KRBQjJy5kYEpYjJxpXp9iU=
go.uber.org/multierr v1.10.0 h1:S0h4aNzvfcFsC3dRF1jLoaov7oRaKqRGC/pUEJ2yvPQ=
go.uber.org/multierr v1.10.0/go.mod h1:20+QtiLqy0Nd6FdQB9TLXag12DsQkrbs3htMFfDN80Y=
go.uber.org/zap v1.18.1/go.mod h1:xg/QME4nWcxGxrpdeYfq7UvYrLh66cuVKdrbD1XF/NI=
go.uber.org/zap v1.27.0 h1:aJMhYGrd5QSmlpLMr2MftRKl7t8J8PTZPA732ud/XR8=
go.uber.org/zap v1.27.0/go.mod h1:GB2qFLM7cTU87MWRP2mPIjqfIDnGu+VIO4V/SdhGo2E=
golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2/go.mod h1:djNgcEr1/C05ACkg1iLfiJU5Ep61QUkGW8qpdssI0+w=
golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550/go.mod h1:yigFU9vqHzYiE8UmvKecakEJjdnWj3jj499lnFckfCI=
golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9/go.mod h1:LzIPMQfyMNhhGPhUkYOs5KpL4U8rLKemX1yGLhDgUto=
golang.org/x/crypto v0.0.0-20210921155107-089bfa567519/go.mod h1:GvvjBRRGRdwPK5ydBHafDWAxML/pGHZbMvKqRZ5+Abc=
golang.org/x/crypto v0.14.0/go.mod h1:MVFd36DqK4CsrnJYDkBA3VC4m2GkXAM0PvzMCn4JQf4=
golang.org/x/crypto v0.30.0 h1:RwoQn3GkWiMkzlX562cLB7OxWvjH1L8xutO2WoJcRoY=
golang.org/x/crypto v0.30.0/go.mod h1:kDsLvtWBEx7MV9tJOj9bnXsPbxwJQ6csT/x4KIN4Ssk=
golang.org/x/exp v0.0.0-20190121172915-509febef88a4/go.mod h1:CJ0aWSM057203Lf6IL+f9T1iT9GByDxfZKAQTCR3kQA=
golang.org/x/lint v0.0.0-20181026193005-c67002cb31c3/go.mod h1:UVdnD1Gm6xHRNCYTkRU2/jEulfH38KcIWyp/GAMgvoE=
golang.org/x/lint v0.0.0-20190227174305-5b3e6a55c961/go.mod h1:wehouNa3lNwaWXcvxsM5YxQ5yQlVC4a0KAMCusXpPoU=
golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
golang.org/x/lint v0.0.0-20190930215403-16217165b5de/go.mod h1:6SW0HCj/g11FgYtHlgUYUwCkIfeOF89ocIRzGO/8vkc=
golang.org/x/mod v0.2.0/go.mod h1:s0Qsj1ACt9ePp/hMypM3fl4fZqREWJwdYDEqhRiZZUA=
golang.org/x/mod v0.3.0/go.mod h1:s0Qsj1ACt9ePp/hMypM3fl4fZqREWJwdYDEqhRiZZUA=
golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4/go.mod h1:jJ57K6gSWd91VN4djpZkiMVwK6gcyfeH4XE8wZrZaV4=
golang.org/x/mod v0.8.0/go.mod h1:iBbtSCu2XBx23ZKBPSOrRkjjQPZFPuis4dIYUhu/chs=
golang.org/x/net v0.0.0-20180724234803-3673e40ba225/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
golang.org/x/net v0.0.0-20180826012351-8a410e7b638d/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
golang.org/x/net v0.0.0-20190213061140-3a22650c66bd/go.mod h1:mL1N/T3taQHkDXs73rZJwtUhF3w3ftmwwsq0BUmARs4=
golang.org/x/net v0.0.0-20190311183353-d8887717615a/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3/go.mod h1:t9HGtf8HONx5eT2rtn7q6eTqICYqUVnKs3thJo3Qplg=
golang.org/x/net v0.0.0-20190620200207-3b0461eec859/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
golang.org/x/net v0.0.0-20200226121028-0de0cce0169b/go.mod h1:z5CRVTTTmAJ677TzLLGU+0bjPO0LkuOLi4/5GtJWs/s=
golang.org/x/net v0.0.0-20201021035429-f5854403a974/go.mod h1:sp8m0HH+o8qH0wwXwYZr8TS3Oi6o0r6Gce1SSxlDquU=
golang.org/x/net v0.0.0-20210226172049-e18ecbb05110/go.mod h1:m0MpNAwzfU5UDzcl9v0D8zg8gWTRqZa9RBIspLL5mdg=
golang.org/x/net v0.0.0-20220722155237-a158d28d115b/go.mod h1:XRhObCWvk6IyKnWLug+ECip1KBveYUHfp+8e9klMJ9c=
golang.org/x/net v0.6.0/go.mod h1:2Tu9+aMcznHK/AK1HMvgo6xiTLG5rD5rZLDS+rp2Bjs=
golang.org/x/net v0.10.0/go.mod h1:0qNGK6F8kojg2nk9dLZ2mShWaEBan6FAoqfSigmmuDg=
golang.org/x/net v0.17.0/go.mod h1:NxSsAGuq816PNPmqtQdLE42eU2Fs7NoRIZrHJAlaCOE=
golang.org/x/net v0.30.0 h1:AcW1SDZMkb8IpzCdQUaIq2sP4sZ4zw+55h6ynffypl4=
golang.org/x/net v0.30.0/go.mod h1:2wGyMJ5iFasEhkwi13ChkO/t1ECNC4X4eBKkVFyYFlU=
golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be/go.mod h1:N/0e6XlmueqKjAGxoOufVs8QHGRruUQn6yWY3a++T0U=
golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20181108010431-42b317875d0f/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20190423024810-112230192c58/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sync v0.1.0/go.mod h1:RxMgew5VJxzue5/jJTE5uejpjVlOe/izrB70Jof72aM=
golang.org/x/sys v0.0.0-20180830151530-49385e6e1522/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
golang.org/x/sys v0.0.0-20190215142949-d0b11bdaac8a/go.mod h1:STP8DvDyc/dI5b8T5hshtkjS+E42TnysNCUPdjciGhY=
golang.org/x/sys v0.0.0-20190412213103-97732733099d/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20190422165155-953cdadca894/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20201119102817-f84b799fce68/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.5.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.6.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.8.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.13.0/go.mod h1:oPkhp1MJrh7nUepCBck5+mAzfO9JrbApNNgaTdGDITg=
golang.org/x/sys v0.28.0 h1:Fksou7UEQUWlKvIdsqzJmUmCX3cZuD2+P3XyyzwMhlA=
golang.org/x/sys v0.28.0/go.mod h1:/VUhepiaJMQUp4+oa/7Zr1D23ma6VTLIYjOOTFZPUcA=
golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1/go.mod h1:bj7SfCRtBDWHUb9snDiAeCFNEtKQo2Wmx5Cou7ajbmo=
golang.org/x/term v0.0.0-20210927222741-03fcf44c2211/go.mod h1:jbD1KX2456YbFQfuXm/mYQcufACuNUgVhRMnK/tPxf8=
golang.org/x/term v0.5.0/go.mod h1:jMB1sMXY+tzblOD4FWmEbocvup2/aLOaQEp7JmGp78k=
golang.org/x/term v0.8.0/go.mod h1:xPskH00ivmX89bAKVGSKKtLOWNx2+17Eiy94tnKShWo=
golang.org/x/term v0.13.0/go.mod h1:LTmsnFJwVN6bCy1rVCoS+qHT1HhALEFxKncY3WNNh4U=
golang.org/x/term v0.27.0 h1:WP60Sv1nlK1T6SupCHbXzSaN0b9wUmsPoRS9b61A23Q=
golang.org/x/term v0.27.0/go.mod h1:iMsnZpn0cago0GOrHO2+Y7u7JPn5AylBrcoWkElMTSM=
golang.org/x/text v0.3.0/go.mod h1:NqM8EUOU14njkJ3fqMW+pc6Ldnwhi/IjpwHt7yyuwOQ=
golang.org/x/text v0.3.3/go.mod h1:5Zoc/QRtKVWzQhOtBMvqHzDpF6irO9z98xDceosuGiQ=
golang.org/x/text v0.3.7/go.mod h1:u+2+/6zg+i71rQMx5EYifcz6MCKuco9NR6JIITiCfzQ=
golang.org/x/text v0.3.8/go.mod h1:E6s5w1FMmriuDzIBO73fBruAKo1PCIq6d2Q6DHfQ8WQ=
golang.org/x/text v0.7.0/go.mod h1:mrYo+phRRbMaCq/xk9113O4dZlRixOauAjOtrjsXDZ8=
golang.org/x/text v0.9.0/go.mod h1:e1OnstbJyHTd6l/uOt8jFFHp6TRDWZR/bV3emEE/zU8=
golang.org/x/text v0.13.0/go.mod h1:TvPlkZtksWOMsz7fbANvkp4WM8x/WCo/om8BMLbz+aE=
golang.org/x/text v0.21.0 h1:zyQAAkrwaneQ066sspRyJaG9VNi/YJ1NfzcGB3hZ/qo=
golang.org/x/text v0.21.0/go.mod h1:4IBbMaMmOPCJ8SecivzSH54+73PCFmPWxNTLm+vZkEQ=
golang.org/x/time v0.5.0 h1:o7cqy6amK/52YcAKIPlM3a+Fpj35zvRj2TP+e1xFSfk=
golang.org/x/time v0.5.0/go.mod h1:3BpzKBy/shNhVucY/MWOyx10tF3SFh9QdLuxbVysPQM=
golang.org/x/tools v0.0.0-20180917221912-90fa682c2a6e/go.mod h1:n7NCudcB/nEzxVGmLbDWY5pfWTLqBcC2KZ6jyYvM4mQ=
golang.org/x/tools v0.0.0-20190114222345-bf090417da8b/go.mod h1:n7NCudcB/nEzxVGmLbDWY5pfWTLqBcC2KZ6jyYvM4mQ=
golang.org/x/tools v0.0.0-20190226205152-f727befe758c/go.mod h1:9Yl7xja0Znq3iFh3HoIrodX9oNMXvdceNzlUR8zjMvY=
golang.org/x/tools v0.0.0-20190311212946-11955173bddd/go.mod h1:LCzVGOaR6xXOjkQ3onu1FJEFr0SW1gC7cKk1uF8kGRs=
golang.org/x/tools v0.0.0-20190524140312-2c0ae7006135/go.mod h1:RgjU9mgBXZiqYHBnxXauZ1Gv1EHHAz9KjViQ78xBX0Q=
golang.org/x/tools v0.0.0-20191108193012-7d206e10da11/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
golang.org/x/tools v0.0.0-20191119224855-298f0cb1881e/go.mod h1:b+2E5dAYhXwXZwtnZ6UAqBI28+e2cm9otk0dWdXHAEo=
golang.org/x/tools v0.0.0-20200619180055-7c47624df98f/go.mod h1:EkVYQZoAsY45+roYkvgYkIh4xh/qjgUK9TdY2XT94GE=
golang.org/x/tools v0.0.0-20210106214847-113979e3529a/go.mod h1:emZCQorbCU4vsT4fOWvOPXz4eW1wZW4PmDk9uLelYpA=
golang.org/x/tools v0.1.12/go.mod h1:hNGJHUnrk76NpqgfD5Aqm5Crs+Hm0VOH/i9J2+nxYbc=
golang.org/x/tools v0.6.0/go.mod h1:Xwgl3UAJ/d3gWutnCtw505GrjyAbvKui8lOU390QaIU=
golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1/go.mod h1:I/5z698sn9Ka8TeJc9MKroUUfqBBauWjQqLJ2OPfmY0=
google.golang.org/appengine v1.1.0/go.mod h1:EbEs0AVv82hx2wNQdGPgUI5lhzA/G0D9YwlJXL52JkM=
google.golang.org/appengine v1.4.0/go.mod h1:xpcJRLb0r/rnEns0DIKYYv+WjYCduHsrkT7/EB5XEv4=
google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8/go.mod h1:JiN7NxoALGmiZfu7CAH4rXhgtRTLTxftemlI0sWmxmc=
google.golang.org/genproto v0.0.0-20190819201941-24fa4b261c55/go.mod h1:DMBHOl98Agz4BDEuKkezgsaosCRResVns1a3J2ZsMNc=
google.golang.org/genproto v0.0.0-20200423170343-7949de9c1215/go.mod h1:55QSHmfGQM9UVYDPBsyGGes0y52j32PQ3BqQfXhyH3c=
google.golang.org/genproto/googleapis/rpc v0.0.0-20241104194629-dd2ea8efbc28 h1:XVhgTWWV3kGQlwJHR3upFWZeTsei6Oks1apkZSeonIE=
google.golang.org/genproto/googleapis/rpc v0.0.0-20241104194629-dd2ea8efbc28/go.mod h1:GX3210XPVPUjJbTUbvwI8f2IpZDMZuPJWDzDuebbviI=
google.golang.org/grpc v1.19.0/go.mod h1:mqu4LbDTu4XGKhr4mRzUsmM4RtVoemTSY81AxZiDr8c=
google.golang.org/grpc v1.23.0/go.mod h1:Y5yQAOtifL1yxbo5wqy6BxZv8vAUGQwXBOALyacEbxg=
google.golang.org/grpc v1.25.1/go.mod h1:c3i+UQWmh7LiEpx4sFZnkU36qjEYZ0imhYfXVyQciAY=
google.golang.org/grpc v1.27.0/go.mod h1:qbnxyOmOxrQa7FizSgH+ReBfzJrCY1pSN7KXBS8abTk=
google.golang.org/grpc v1.29.1/go.mod h1:itym6AZVZYACWQqET3MqgPpjcuV5QH3BxFS3IjizoKk=
google.golang.org/grpc v1.69.2 h1:U3S9QEtbXC0bYNvRtcoklF3xGtLViumSYxWykJS+7AU=
google.golang.org/grpc v1.69.2/go.mod h1:vyjdE6jLBI76dgpDojsFGNaHlxdjXN9ghpnd2o7JGZ4=
google.golang.org/protobuf v1.36.0 h1:mjIs9gYtt56AzC4ZaffQuh88TZurBGhIJMBZGSxNerQ=
google.golang.org/protobuf v1.36.0/go.mod h1:9fA7Ob0pmnwhb644+1+CVWFRbNajQ6iRojtC/QF5bRE=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 h1:qIbj1fsPNlZgppZ+VLlY7N33q108Sa+fhmuc+sWQYwY=
gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
gopkg.in/yaml.v2 v2.2.1/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
gopkg.in/yaml.v2 v2.2.2/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
gopkg.in/yaml.v2 v2.2.8/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
gopkg.in/yaml.v2 v2.4.0 h1:D8xgwECY7CYvx+Y2n4sBz93Jn9JRvxdiyyo8CTfuKaY=
gopkg.in/yaml.v2 v2.4.0/go.mod h1:RDklbk79AGWmwhnvt/jBztapEOGDOx6ZbXqjP6csGnQ=
gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
honnef.co/go/tools v0.0.0-20190102054323-c2f93a96b099/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=
honnef.co/go/tools v0.0.0-20190523083050-ea95bdfd59fc/go.mod h1:rf3lG4BRIbNafJWhAfAdb/ePZxsR/4RtNHQocxwk9r4=


// golangci.yaml
linters:
  disable-all: true
  enable:
    - deadcode
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - structcheck
    - typecheck
    - unused
    - varcheck
    - asciicheck
    - bodyclose
    - dogsled
    - exhaustive
    - exportloopref
    - gocognit
    - goconst
    - gofmt
    - goheader
    - goimports
    - gosec
    - misspell
    - nakedret
    - nestif
    - noctx
    - rowserrcheck
    - sqlclosecheck
    - unconvert
    - unparam
    - whitespace

issues:
  exclude:
    - "composite literal uses unkeyed fields"
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec
        - noctx
        - unparam
        - bodyclose
    - path: fixtures.go
      linters:
        - gosec

// internal/article/configurator/article_configurator.go
package articleConfigurator

import (
	"context"
	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"

	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	articleGrpcController "github.com/diki-haryadi/go-micro-template/internal/article/delivery/grpc"
	articleHttpController "github.com/diki-haryadi/go-micro-template/internal/article/delivery/http"
	articleKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/article/delivery/kafka/producer"
	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	articleRepository "github.com/diki-haryadi/go-micro-template/internal/article/repository"
	articleUseCase "github.com/diki-haryadi/go-micro-template/internal/article/usecase"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	infraContainer "github.com/diki-haryadi/ztools/infra_container"
)

type configurator struct {
	ic        *infraContainer.IContainer
	extBridge *externalBridge.ExternalBridge
}

func NewConfigurator(ic *infraContainer.IContainer, extBridge *externalBridge.ExternalBridge) articleDomain.Configurator {
	return &configurator{ic: ic, extBridge: extBridge}
}

func (c *configurator) Configure(ctx context.Context) error {
	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(c.extBridge.SampleExtGrpcService)
	kafkaProducer := articleKafkaProducer.NewProducer(c.ic.KafkaWriter)
	repository := articleRepository.NewRepository(c.ic.Postgres)
	useCase := articleUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// grpc
	grpcController := articleGrpcController.NewController(useCase)
	articleV1.RegisterArticleServiceServer(c.ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	// http
	httpRouterGp := c.ic.EchoHttpServer.GetEchoInstance().Group(c.ic.EchoHttpServer.GetBasePath())
	httpController := articleHttpController.NewController(useCase)
	articleHttpController.NewRouter(httpController).Register(httpRouterGp)

	// consumers
	//articleKafkaConsumer.NewConsumer(c.ic.KafkaReader).RunConsumers(ctx)

	// jobs
	//articleJob.NewJob(c.ic.Logger).StartJobs(ctx)

	return nil
}


// internal/article/delivery/grpc/article_grpc_controller.go
package articleGrpcController

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	articleException "github.com/diki-haryadi/go-micro-template/internal/article/exception"
)

type controller struct {
	useCase articleDomain.UseCase
}

func NewController(uc articleDomain.UseCase) articleDomain.GrpcController {
	return &controller{
		useCase: uc,
	}
}

func (c *controller) CreateArticle(ctx context.Context, req *articleV1.CreateArticleRequest) (*articleV1.CreateArticleResponse, error) {
	aDto := &articleDto.CreateArticleRequestDto{
		Name:        req.Name,
		Description: req.Desc,
	}
	err := aDto.ValidateCreateArticleDto()
	if err != nil {
		return nil, articleException.CreateArticleValidationExc(err)
	}

	article, err := c.useCase.CreateArticle(ctx, aDto)
	if err != nil {
		return nil, err
	}

	return &articleV1.CreateArticleResponse{
		Id:   article.ID.String(),
		Name: article.Name,
		Desc: article.Description,
	}, nil
}

func (c *controller) GetArticleById(ctx context.Context, req *articleV1.GetArticleByIdRequest) (*articleV1.GetArticleByIdResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}


// internal/article/delivery/http/article_http_controller.go
package articleHttpController

import (
	"net/http"

	"github.com/labstack/echo/v4"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	articleException "github.com/diki-haryadi/go-micro-template/internal/article/exception"
)

type controller struct {
	useCase articleDomain.UseCase
}

func NewController(uc articleDomain.UseCase) articleDomain.HttpController {
	return &controller{
		useCase: uc,
	}
}

func (c controller) CreateArticle(ctx echo.Context) error {
	aDto := new(articleDto.CreateArticleRequestDto)
	if err := ctx.Bind(aDto); err != nil {
		return articleException.ArticleBindingExc()
	}

	if err := aDto.ValidateCreateArticleDto(); err != nil {
		return articleException.CreateArticleValidationExc(err)
	}

	article, err := c.useCase.CreateArticle(ctx.Request().Context(), aDto)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, article)
}


// internal/article/delivery/http/article_http_router.go
package articleHttpController

import (
	"github.com/labstack/echo/v4"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
)

type Router struct {
	controller articleDomain.HttpController
}

func NewRouter(controller articleDomain.HttpController) *Router {
	return &Router{
		controller: controller,
	}
}

func (r *Router) Register(e *echo.Group) {
	e.POST("/article", r.controller.CreateArticle)
}


// internal/article/delivery/kafka/consumer/consumer.go
package articleKafkaConsumer

import (
	"context"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	kafkaConsumer "github.com/diki-haryadi/ztools/kafka/consumer"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"
)

type consumer struct {
	createEventReader *kafkaConsumer.Reader
}

func NewConsumer(r *kafkaConsumer.Reader) articleDomain.KafkaConsumer {
	return &consumer{createEventReader: r}
}

func (c *consumer) RunConsumers(ctx context.Context) {
	go c.createEvent(ctx, 2)
}

func (c *consumer) createEvent(ctx context.Context, workersNum int) {
	r := c.createEventReader.Client
	defer func() {
		if err := r.Close(); err != nil {
			logger.Zap.Sugar().Errorf("error closing create article consumer")
		}
	}()

	logger.Zap.Sugar().Infof("Starting consumer group: %v", r.Config().GroupID)

	workerChan := make(chan bool)
	worker := wrapper.BuildChain(
		c.createEventWorker(workerChan),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)
	for i := 0; i <= workersNum; i++ {
		go worker.ToWorkerFunc(ctx, nil)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-workerChan:
			go worker.ToWorkerFunc(ctx, nil)
		}
	}
}


// internal/article/delivery/kafka/consumer/worker.go
package articleKafkaConsumer

import (
	"context"
	"encoding/json"

	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
)

func (c *consumer) createEventWorker(
	workerChan chan bool,
) wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		defer func() {
			workerChan <- true
		}()
		for {
			msg, err := c.createEventReader.Client.FetchMessage(ctx)
			if err != nil {
				return nil, err
			}

			logger.Zap.Sugar().Infof(
				"Kafka Worker recieved message at topic/partition/offset %v/%v/%v: %s = %s\n",
				msg.Topic,
				msg.Partition,
				msg.Offset,
				string(msg.Key),
				string(msg.Value),
			)

			aDto := new(articleDto.CreateArticleRequestDto)
			if err := json.Unmarshal(msg.Value, &aDto); err != nil {
				continue
			}

			if err := c.createEventReader.Client.CommitMessages(ctx, msg); err != nil {
				return nil, err
			}
		}
	}
}


// internal/article/delivery/kafka/producer/producer.go
package articleKafkaProducer

import (
	"context"

	"github.com/segmentio/kafka-go"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	kafkaProducer "github.com/diki-haryadi/ztools/kafka/producer"
)

type producer struct {
	createWriter *kafkaProducer.Writer
}

func NewProducer(w *kafkaProducer.Writer) articleDomain.KafkaProducer {
	return &producer{createWriter: w}
}

func (p *producer) PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error {
	return p.createWriter.Client.WriteMessages(ctx, messages...)
}


// internal/article/domain/article_domain.go
package articleDomain

import (
	"context"

	"github.com/google/uuid"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"

	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
)

type Article struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"desc"`
}

type Configurator interface {
	Configure(ctx context.Context) error
}

type UseCase interface {
	CreateArticle(ctx context.Context, article *articleDto.CreateArticleRequestDto) (*articleDto.CreateArticleResponseDto, error)
}

type Repository interface {
	CreateArticle(ctx context.Context, article *articleDto.CreateArticleRequestDto) (*articleDto.CreateArticleResponseDto, error)
}

type GrpcController interface {
	CreateArticle(ctx context.Context, req *articleV1.CreateArticleRequest) (*articleV1.CreateArticleResponse, error)
	GetArticleById(ctx context.Context, req *articleV1.GetArticleByIdRequest) (*articleV1.GetArticleByIdResponse, error)
}

type HttpController interface {
	CreateArticle(c echo.Context) error
}

type Job interface {
	StartJobs(ctx context.Context)
}

type KafkaProducer interface {
	PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error
}

type KafkaConsumer interface {
	RunConsumers(ctx context.Context)
}


// internal/article/dto/create_article_dto.go
package articleDto

import (
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
)

type CreateArticleRequestDto struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
}

func (caDto *CreateArticleRequestDto) ValidateCreateArticleDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.Name,
			validator.Required,
			validator.Length(3, 50),
		),
		validator.Field(
			&caDto.Description,
			validator.Required,
			validator.Length(5, 100),
		),
	)
}

type CreateArticleResponseDto struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
}


// internal/article/exception/article_exception.go
package articleException

import (
	errorList "github.com/diki-haryadi/ztools/constant/error/error_list"
	customErrors "github.com/diki-haryadi/ztools/error/custom_error"
	errorUtils "github.com/diki-haryadi/ztools/error/error_utils"
)

func CreateArticleValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func ArticleBindingExc() error {
	articleBindingError := errorList.InternalErrorList.ArticleExceptions.BindingError
	return customErrors.NewBadRequestError(articleBindingError.Msg, articleBindingError.Code, nil)
}


// internal/article/job/job.go
package articleJob

import (
	"context"

	"go.uber.org/zap"

	"github.com/robfig/cron/v3"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"

	cronJob "github.com/diki-haryadi/ztools/cron"
)

type job struct {
	cron   *cron.Cron
	logger *zap.Logger
}

func NewJob(logger *zap.Logger) articleDomain.Job {
	newCron := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cronJob.NewLogger()),
	))
	return &job{cron: newCron, logger: logger}
}

func (j *job) StartJobs(ctx context.Context) {
	j.logArticleJob(ctx)
	go j.cron.Start()
}

func (j *job) logArticleJob(ctx context.Context) {
	worker := wrapper.BuildChain(j.logArticleWorker(),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)

	entryId, _ := j.cron.AddFunc("*/1 * * * *",
		worker.ToWorkerFunc(ctx, nil),
	)

	j.logger.Sugar().Infof("Article Job Started: %v", entryId)
}


// internal/article/job/worker.go
package articleJob

import (
	"context"

	"github.com/diki-haryadi/ztools/wrapper"
)

func (j *job) logArticleWorker() wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		j.logger.Info("article log job")
		return nil, nil
	}
}


// internal/article/repository/article_repo.go
package articleRepository

import (
	"context"
	"fmt"

	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	"github.com/diki-haryadi/ztools/postgres"
)

type repository struct {
	postgres *postgres.Postgres
}

func NewRepository(conn *postgres.Postgres) articleDomain.Repository {
	return &repository{postgres: conn}
}

func (rp *repository) CreateArticle(
	ctx context.Context,
	entity *articleDto.CreateArticleRequestDto,
) (*articleDto.CreateArticleResponseDto, error) {
	query := `INSERT INTO articles (name, description) VALUES ($1, $2) RETURNING id, name, description`

	result, err := rp.postgres.SqlxDB.QueryContext(ctx, query, entity.Name, entity.Description)
	if err != nil {
		return nil, fmt.Errorf("error inserting article record")
	}

	article := new(articleDto.CreateArticleResponseDto)
	for result.Next() {
		err = result.Scan(&article.ID, &article.Name, &article.Description)
		if err != nil {
			return nil, err
		}
	}

	return article, nil
}


// internal/article/tests/fixtures/article_integration_fixture.go
package articleFixture

import (
	"context"
	"math"
	"net"
	"time"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	articleGrpc "github.com/diki-haryadi/go-micro-template/internal/article/delivery/grpc"
	articleHttp "github.com/diki-haryadi/go-micro-template/internal/article/delivery/http"
	articleKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/article/delivery/kafka/producer"
	articleRepo "github.com/diki-haryadi/go-micro-template/internal/article/repository"
	articleUseCase "github.com/diki-haryadi/go-micro-template/internal/article/usecase"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	iContainer "github.com/diki-haryadi/ztools/infra_container"
	"github.com/diki-haryadi/ztools/logger"
)

const BUFSIZE = 1024 * 1024

type IntegrationTestFixture struct {
	TearDown          func()
	Ctx               context.Context
	Cancel            context.CancelFunc
	InfraContainer    *iContainer.IContainer
	ArticleGrpcClient articleV1.ArticleServiceClient
}

func NewIntegrationTestFixture() (*IntegrationTestFixture, error) {
	deadline := time.Now().Add(time.Duration(math.MaxInt64))
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	ic, infraDown, err := iContainer.NewIC(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	extBridge, extBridgeDown, err := externalBridge.NewExternalBridge(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(extBridge.SampleExtGrpcService)
	kafkaProducer := articleKafkaProducer.NewProducer(ic.KafkaWriter)
	repository := articleRepo.NewRepository(ic.Postgres)
	useCase := articleUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// http
	ic.EchoHttpServer.SetupDefaultMiddlewares()
	httpRouterGp := ic.EchoHttpServer.GetEchoInstance().Group(ic.EchoHttpServer.GetBasePath())
	httpController := articleHttp.NewController(useCase)
	articleHttp.NewRouter(httpController).Register(httpRouterGp)

	// grpc
	grpcController := articleGrpc.NewController(useCase)
	articleV1.RegisterArticleServiceServer(ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	lis := bufconn.Listen(BUFSIZE)
	go func() {
		if err := ic.GrpcServer.GetCurrentGrpcServer().Serve(lis); err != nil {
			logger.Zap.Sugar().Fatalf("Server exited with error: %v", err)
		}
	}()

	grpcClientConn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	articleGrpcClient := articleV1.NewArticleServiceClient(grpcClientConn)

	return &IntegrationTestFixture{
		TearDown: func() {
			cancel()
			infraDown()
			_ = grpcClientConn.Close()
			extBridgeDown()
		},
		InfraContainer:    ic,
		Ctx:               ctx,
		Cancel:            cancel,
		ArticleGrpcClient: articleGrpcClient,
	}, nil
}


// internal/article/tests/integrations/create_article_test.go
package artcileIntegrationTest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"github.com/labstack/echo/v4"

	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	articleFixture "github.com/diki-haryadi/go-micro-template/internal/article/tests/fixtures"
	grpcError "github.com/diki-haryadi/ztools/error/grpc"
	httpError "github.com/diki-haryadi/ztools/error/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type testSuite struct {
	suite.Suite
	fixture *articleFixture.IntegrationTestFixture
}

func (suite *testSuite) SetupSuite() {
	fixture, err := articleFixture.NewIntegrationTestFixture()
	if err != nil {
		assert.Error(suite.T(), err)
	}

	suite.fixture = fixture
}

func (suite *testSuite) TearDownSuite() {
	suite.fixture.TearDown()
}

func (suite *testSuite) TestSuccessfulCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro Developer",
	}

	response, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)
	if err != nil {
		assert.Error(suite.T(), err)
	}

	assert.NotNil(suite.T(), response.Id)
	assert.Equal(suite.T(), "John", response.Name)
	assert.Equal(suite.T(), "Pro Developer", response.Desc)
}

func (suite *testSuite) TestNameValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "Jo",
		Desc: "Pro Developer",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "name")
}

func (suite *testSuite) TestDescValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "desc")
}

func (suite *testSuite) TestSuccessCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusOK, response.Code)

	caDto := new(articleDto.CreateArticleRequestDto)
	if assert.NoError(suite.T(), json.Unmarshal(response.Body.Bytes(), caDto)) {
		assert.Equal(suite.T(), "John Snow", caDto.Name)
		assert.Equal(suite.T(), "King of the north", caDto.Description)
	}

}

func (suite *testSuite) TestNameValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"Jo","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "name")
	}

}

func (suite *testSuite) TestDescValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()

	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "desc")
	}
}

func TestRunSuite(t *testing.T) {
	tSuite := new(testSuite)
	suite.Run(t, tSuite)
}


// internal/article/usecase/article_usecase.go
package articleUseCase

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"

	sampleExtServiceDomain "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/domain"
	articleDomain "github.com/diki-haryadi/go-micro-template/internal/article/domain"
	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
)

type useCase struct {
	repository              articleDomain.Repository
	kafkaProducer           articleDomain.KafkaProducer
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase
}

func NewUseCase(
	repository articleDomain.Repository,
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase,
	kafkaProducer articleDomain.KafkaProducer,
) articleDomain.UseCase {
	return &useCase{
		repository:              repository,
		kafkaProducer:           kafkaProducer,
		sampleExtServiceUseCase: sampleExtServiceUseCase,
	}
}

func (uc *useCase) CreateArticle(ctx context.Context, req *articleDto.CreateArticleRequestDto) (*articleDto.CreateArticleResponseDto, error) {
	article, err := uc.repository.CreateArticle(ctx, req)
	if err != nil {
		return nil, err
	}

	// TODO : if err => return Marshal_Err_Exception
	jsonArticle, _ := json.Marshal(article)

	// if it has go keyword and if we pass the request context to it, it will terminate after request lifecycle.
	_ = uc.kafkaProducer.PublishCreateEvent(context.Background(), kafka.Message{
		Key:   []byte("Article"),
		Value: jsonArticle,
	})

	return article, err
}


// internal/authentication/configurator/auth_configurator.go
package authConfigurator

import (
	"context"
	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	authGrpcController "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/grpc"
	authHttpController "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/http"
	authKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/kafka/producer"
	authDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	authRepository "github.com/diki-haryadi/go-micro-template/internal/authentication/repository"
	authUseCase "github.com/diki-haryadi/go-micro-template/internal/authentication/usecase"
	authenticationV1 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/authentication/v1"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	infraContainer "github.com/diki-haryadi/ztools/infra_container"
)

type configurator struct {
	ic        *infraContainer.IContainer
	extBridge *externalBridge.ExternalBridge
}

func NewConfigurator(ic *infraContainer.IContainer, extBridge *externalBridge.ExternalBridge) authDomain.Configurator {
	return &configurator{ic: ic, extBridge: extBridge}
}

func (c *configurator) Configure(ctx context.Context) error {
	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(c.extBridge.SampleExtGrpcService)
	kafkaProducer := authKafkaProducer.NewProducer(c.ic.KafkaWriter)
	repository := authRepository.NewRepository(c.ic.Postgres)
	useCase := authUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// grpc
	grpcController := authGrpcController.NewController(useCase)
	authenticationV1.RegisterAuthServiceServer(c.ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	// http
	httpRouterGp := c.ic.EchoHttpServer.GetEchoInstance().Group(c.ic.EchoHttpServer.GetBasePath())
	httpController := authHttpController.NewController(useCase)
	authHttpController.NewRouter(httpController).Register(httpRouterGp)

	// consumers
	//oauthKafkaConsumer.NewConsumer(c.ic.KafkaReader).RunConsumers(ctx)

	// jobs
	//oauthJob.NewJob(c.ic.Logger).StartJobs(ctx)

	return nil
}


// internal/authentication/delivery/grpc/auth_grpc_controller.go
package authGrpcController

import (
	"context"
	authDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	authModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	authDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	auhenticationV1 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/authentication/v1"
	"github.com/google/uuid"
)

type controller struct {
	useCase authDomain.UseCase
}

func NewController(uc authDomain.UseCase) authDomain.GrpcController {
	return &controller{
		useCase: uc,
	}
}

func (c *controller) BasicAuthClient(ctx context.Context, clientID, secret string) (*authModel.Client, error) {

	// Authenticate the client
	client, err := c.useCase.AuthClient(ctx, clientID, secret)
	if err != nil {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	return client, nil
}

func (c *controller) Register(ctx context.Context, req *auhenticationV1.RegisterRequest) (*auhenticationV1.RegisterResponse, error) {
	aDto := new(authDto.UserRequestDto).GetFieldsUserValue(req.Username, req.Password, req.RoleId)
	if err := aDto.ValidateUserDto(); err != nil {
		return &auhenticationV1.RegisterResponse{}, err
	}

	_, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &auhenticationV1.RegisterResponse{}, err
	}

	register, err := c.useCase.Register(
		ctx,
		aDto)

	if err != nil {
		return &auhenticationV1.RegisterResponse{}, err
	}

	return &auhenticationV1.RegisterResponse{
		Uuid:     register.UUID,
		Username: register.Username,
		RoleId:   register.RoleID,
	}, err
}

func (c *controller) ChangePassword(ctx context.Context, req *auhenticationV1.ChangePasswordRequest) (*auhenticationV1.ChangePasswordResponse, error) {
	aDto := new(authDto.ChangePasswordRequest).GetFieldsChangePasswordValue(req.Uuid, req.Password, req.NewPassword)
	if err := aDto.ValidateChangePasswordDto(); err != nil {
		return &auhenticationV1.ChangePasswordResponse{}, err
	}

	_, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &auhenticationV1.ChangePasswordResponse{}, err
	}

	forgotPass, err := c.useCase.ChangePassword(
		ctx, aDto)

	if err != nil {
		return &auhenticationV1.ChangePasswordResponse{}, err
	}
	return &auhenticationV1.ChangePasswordResponse{
		Status: forgotPass.Status,
	}, err
}

func (c *controller) ForgotPassword(ctx context.Context, req *auhenticationV1.ForgotPasswordRequest) (*auhenticationV1.ForgotPasswordResponse, error) {
	aDto := new(authDto.ForgotPasswordRequest).GetFieldsForgotPasswordValue(req.Uuid, req.Password)
	if err := aDto.ValidateForgotPasswordDto(); err != nil {
		return &auhenticationV1.ForgotPasswordResponse{}, err
	}

	_, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &auhenticationV1.ForgotPasswordResponse{}, err
	}

	forgotPass, err := c.useCase.ForgotPassword(
		ctx, aDto)

	if err != nil {
		return &auhenticationV1.ForgotPasswordResponse{}, err
	}
	return &auhenticationV1.ForgotPasswordResponse{
		Status: forgotPass.Status,
	}, err
}

func (c *controller) UpdateUsername(ctx context.Context, req *auhenticationV1.UpdateUsernameRequest) (*auhenticationV1.UpdateUsernameResponse, error) {
	aDto := new(authDto.UpdateUsernameRequest).GetFieldsUpdateUsernameValue(req.Uuid, req.Username)
	if err := aDto.ValidateUsernameDto(); err != nil {
		return &auhenticationV1.UpdateUsernameResponse{}, err
	}

	_, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &auhenticationV1.UpdateUsernameResponse{
			Status: false,
		}, err
	}

	uuid, err := uuid.Parse(aDto.UUID)
	if err != nil {
		return &auhenticationV1.UpdateUsernameResponse{
			Status: false,
		}, err
	}

	err = c.useCase.UpdateUsername(
		ctx,
		aDto.ToModel(uuid), aDto.Username)

	if err != nil {
		return &auhenticationV1.UpdateUsernameResponse{
			Status: false,
		}, err
	}

	return &auhenticationV1.UpdateUsernameResponse{
		Status: true,
	}, err
}


// internal/authentication/delivery/http/auth_http_controller.go
package authHttpController

import (
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type controller struct {
	useCase oauthDomain.UseCase
}

func NewController(uc oauthDomain.UseCase) oauthDomain.HttpController {
	return &controller{
		useCase: uc,
	}
}

func (c controller) BasicAuthClient(ctx echo.Context) (*oauthModel.Client, error) {
	// Get client credentials from basic auth
	clientID, secret, ok := ctx.Request().BasicAuth()
	if !ok {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	// Authenticate the client
	client, err := c.useCase.AuthClient(ctx.Request().Context(), clientID, secret)
	if err != nil {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	return client, nil
}

func (c controller) Register(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.UserRequestDto).GetFieldsUser(ctx)
	if err := aDto.ValidateUserDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	register, err := c.useCase.Register(
		ctx.Request().Context(),
		aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*register).Send(ctx.Response().Writer)
	return nil
}

func (c controller) ChangePassword(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.ChangePasswordRequest).GetFieldsChangePassword(ctx)
	if err := aDto.ValidateChangePasswordDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	changePass, err := c.useCase.ChangePassword(
		ctx.Request().Context(), aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*changePass).Send(ctx.Response().Writer)
	return nil
}

func (c controller) ForgotPassword(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.ForgotPasswordRequest).GetFieldsForgotPassword(ctx)
	if err := aDto.ValidateForgotPasswordDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	forgotPass, err := c.useCase.ForgotPassword(
		ctx.Request().Context(), aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*forgotPass).Send(ctx.Response().Writer)
	return nil
}

func (c controller) UpdateUsername(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.UpdateUsernameRequest).GetFieldsUpdateUsername(ctx)
	if err := aDto.ValidateUsernameDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	uuid, err := uuid.Parse(aDto.UUID)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	err = c.useCase.UpdateUsername(
		ctx.Request().Context(),
		aDto.ToModel(uuid), aDto.Username)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().Send(ctx.Response().Writer)
	return nil
}


// internal/authentication/delivery/http/auth_http_router.go
package authHttpController

import (
	"github.com/labstack/echo/v4"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
)

type Router struct {
	controller oauthDomain.HttpController
}

func NewRouter(controller oauthDomain.HttpController) *Router {
	return &Router{
		controller: controller,
	}
}

func (r *Router) Register(e *echo.Group) {
	auth := e.Group("/auth")
	{
		auth.POST("/register", r.controller.Register)
		auth.POST("/change-password", r.controller.ChangePassword)
		auth.POST("/forgot-password", r.controller.ForgotPassword)
		auth.POST("/update-username", r.controller.UpdateUsername)
	}

}


// internal/authentication/delivery/kafka/consumer/consumer.go
package authKafkaConsumer

import (
	"context"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	kafkaConsumer "github.com/diki-haryadi/ztools/kafka/consumer"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"
)

type consumer struct {
	createEventReader *kafkaConsumer.Reader
}

func NewConsumer(r *kafkaConsumer.Reader) oauthDomain.KafkaConsumer {
	return &consumer{createEventReader: r}
}

func (c *consumer) RunConsumers(ctx context.Context) {
	go c.createEvent(ctx, 2)
}

func (c *consumer) createEvent(ctx context.Context, workersNum int) {
	r := c.createEventReader.Client
	defer func() {
		if err := r.Close(); err != nil {
			logger.Zap.Sugar().Errorf("error closing create article consumer")
		}
	}()

	logger.Zap.Sugar().Infof("Starting consumer group: %v", r.Config().GroupID)

	workerChan := make(chan bool)
	worker := wrapper.BuildChain(
		c.createEventWorker(workerChan),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)
	for i := 0; i <= workersNum; i++ {
		go worker.ToWorkerFunc(ctx, nil)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-workerChan:
			go worker.ToWorkerFunc(ctx, nil)
		}
	}
}


// internal/authentication/delivery/kafka/consumer/worker.go
package authKafkaConsumer

import (
	"context"
	"encoding/json"

	oauthDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
)

func (c *consumer) createEventWorker(
	workerChan chan bool,
) wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		defer func() {
			workerChan <- true
		}()
		for {
			msg, err := c.createEventReader.Client.FetchMessage(ctx)
			if err != nil {
				return nil, err
			}

			logger.Zap.Sugar().Infof(
				"Kafka Worker recieved message at topic/partition/offset %v/%v/%v: %s = %s\n",
				msg.Topic,
				msg.Partition,
				msg.Offset,
				string(msg.Key),
				string(msg.Value),
			)

			aDto := new(oauthDto.CreateArticleRequestDto)
			if err := json.Unmarshal(msg.Value, &aDto); err != nil {
				continue
			}

			if err := c.createEventReader.Client.CommitMessages(ctx, msg); err != nil {
				return nil, err
			}
		}
	}
}


// internal/authentication/delivery/kafka/producer/producer.go
package authKafkaProducer

import (
	"context"

	"github.com/segmentio/kafka-go"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	kafkaProducer "github.com/diki-haryadi/ztools/kafka/producer"
)

type producer struct {
	createWriter *kafkaProducer.Writer
}

func NewProducer(w *kafkaProducer.Writer) oauthDomain.KafkaProducer {
	return &producer{createWriter: w}
}

func (p *producer) PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error {
	return p.createWriter.Client.WriteMessages(ctx, messages...)
}


// internal/authentication/domain/auth_domain.go
package authDomain

import (
	"context"
	model "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	authDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	auhenticationV1 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/authentication/v1"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
)

type Configurator interface {
	Configure(ctx context.Context) error
}

type UseCase interface {
	AuthClient(ctx context.Context, clientID, secret string) (*model.Client, error)
	Register(ctx context.Context, dto *authDto.UserRequestDto) (*authDto.UserResponse, error)
	ChangePassword(ctx context.Context, dto *authDto.ChangePasswordRequest) (*authDto.ChangePasswordResponse, error)
	ForgotPassword(ctx context.Context, dto *authDto.ForgotPasswordRequest) (*authDto.ForgotPasswordResponse, error)
	UpdateUsername(ctx context.Context, user *model.Users, username string) error
}

type Repository interface {
	FetchUserByUserID(ctx context.Context, userID string) (*model.Users, error)
	CreateClientCommon(ctx context.Context, clientID, secret, redirectURI string) (*model.Client, error)
	FindClientByClientID(ctx context.Context, clientID string) (*model.Client, error)
	FindRoleByID(ctx context.Context, id string) (*model.Role, error)
	GetScope(ctx context.Context, requestedScope string) (string, error)
	GetDefaultScope(ctx context.Context) string
	ScopeExists(ctx context.Context, requestedScope string) bool
	FindUserByUsername(ctx context.Context, username string) (*model.Users, error)
	CreateUserCommon(ctx context.Context, roleID, username, password string) (*model.Users, error)
	SetPasswordCommon(ctx context.Context, user *model.Users, password string) error
	UpdateUsernameCommon(ctx context.Context, user *model.Users, username string) error
	UpdatePassword(ctx context.Context, uuid, password string) error
}

type GrpcController interface {
	Register(ctx context.Context, req *auhenticationV1.RegisterRequest) (*auhenticationV1.RegisterResponse, error)
	ChangePassword(ctx context.Context, req *auhenticationV1.ChangePasswordRequest) (*auhenticationV1.ChangePasswordResponse, error)
	ForgotPassword(ctx context.Context, req *auhenticationV1.ForgotPasswordRequest) (*auhenticationV1.ForgotPasswordResponse, error)
	UpdateUsername(ctx context.Context, req *auhenticationV1.UpdateUsernameRequest) (*auhenticationV1.UpdateUsernameResponse, error)
}

type HttpController interface {
	Register(c echo.Context) error
	ChangePassword(c echo.Context) error
	ForgotPassword(c echo.Context) error
	UpdateUsername(ctx echo.Context) error
}

type Job interface {
	StartJobs(ctx context.Context)
}

type KafkaProducer interface {
	PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error
}

type KafkaConsumer interface {
	RunConsumers(ctx context.Context)
}


// internal/authentication/domain/model/client.go
package authDomain

import "database/sql"

type Client struct {
	Common
	Key         string         `db:"key" json:"key"`
	Secret      string         `db:"secret" json:"secret"`
	RedirectURI sql.NullString `db:"redirect_uri" json:"redirect_uri"`
}


// internal/authentication/domain/model/common.go
package authDomain

import (
	"github.com/google/uuid"
	"time"
)

type Common struct {
	ID        uuid.UUID  `db:"id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type Timestamp struct {
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type EmailTokenModel struct {
	Common
	Reference   string     `db:"reference"`
	EmailSent   bool       `db:"email_sent"`
	EmailSentAt *time.Time `db:"email_sent_at"`
	ExpiresAt   time.Time  `db:"expires_at"`
}


// internal/authentication/domain/model/role.go
package authDomain

import "github.com/google/uuid"

type Role struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
	Timestamp
}


// internal/authentication/domain/model/user.go
package authDomain

import "database/sql"

type Users struct {
	Common
	RoleID   sql.NullString `db:"role_id" json:"role_id"`
	Role     *Role
	Username string         `db:"username" json:"username"`
	Password sql.NullString `db:"password" json:"password"`
}


// internal/authentication/dto/change_password.go
package authDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ChangePasswordRequest struct {
	UUID        string `json:"uuid"`
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

func (g *ChangePasswordRequest) GetFieldsChangePassword(ctx echo.Context) *ChangePasswordRequest {
	return &ChangePasswordRequest{
		UUID:        ctx.FormValue("uuid"),
		Password:    ctx.FormValue("password"),
		NewPassword: ctx.FormValue("new_password"),
	}
}

func (g *ChangePasswordRequest) GetFieldsChangePasswordValue(uuid, password, new_password string) *ChangePasswordRequest {
	return &ChangePasswordRequest{
		UUID:        uuid,
		Password:    password,
		NewPassword: new_password,
	}
}

func (g *ChangePasswordRequest) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *ChangePasswordRequest) ValidateChangePasswordDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
		validator.Field(
			&caDto.NewPassword,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type ChangePasswordResponse struct {
	Status bool `json:"status"`
}


// internal/authentication/dto/forgot_password.go
package authDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ForgotPasswordRequest struct {
	UUID     string `json:"uuid"`
	Password string `json:"password"`
}

func (g *ForgotPasswordRequest) GetFieldsForgotPassword(ctx echo.Context) *ForgotPasswordRequest {
	return &ForgotPasswordRequest{
		UUID:     ctx.FormValue("uuid"),
		Password: ctx.FormValue("password"),
	}
}

func (g *ForgotPasswordRequest) GetFieldsForgotPasswordValue(uuid, password string) *ForgotPasswordRequest {
	return &ForgotPasswordRequest{
		UUID:     uuid,
		Password: password,
	}
}

func (g *ForgotPasswordRequest) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *ForgotPasswordRequest) ValidateForgotPasswordDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type ForgotPasswordResponse struct {
	Status bool `json:"status"`
}


// internal/authentication/dto/jwt_token.go
package authDto

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id,omitempty"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	TokenType string `json:"token_type"`
}

func generateJWTToken(claims TokenClaims, secretKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateToken(tokenString, secretKey string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}


// internal/authentication/dto/register_dto.go
package authDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type UserRequestDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   string `json:"role_id"`
}

func (g *UserRequestDto) GetFieldsUser(ctx echo.Context) *UserRequestDto {
	return &UserRequestDto{
		Username: ctx.FormValue("username"),
		Password: ctx.FormValue("password"),
		RoleID:   ctx.FormValue("role_id"),
	}
}

func (g *UserRequestDto) GetFieldsUserValue(username, password, role_id string) *UserRequestDto {
	return &UserRequestDto{
		Username: username,
		Password: password,
		RoleID:   role_id,
	}
}

func (g *UserRequestDto) ToModelUser(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *UserRequestDto) ValidateUserDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.Username,
			validator.Required,
			validator.Length(5, 50),
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
		validator.Field(
			&caDto.RoleID,
			validator.Required,
		),
	)
}

// UserResponse ...
type UserResponse struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   string `json:"role_id"`
}


// internal/authentication/dto/update_username.go
package authDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type UpdateUsernameRequest struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

func (g *UpdateUsernameRequest) GetFieldsUpdateUsername(ctx echo.Context) *UpdateUsernameRequest {
	return &UpdateUsernameRequest{
		UUID:     ctx.FormValue("uuid"),
		Username: ctx.FormValue("username"),
	}
}

func (g *UpdateUsernameRequest) GetFieldsUpdateUsernameValue(uuid, username string) *UpdateUsernameRequest {
	return &UpdateUsernameRequest{
		UUID:     uuid,
		Username: username,
	}
}

func (g *UpdateUsernameRequest) ToModel(userID uuid.UUID) *oauthModel.Users {
	return &oauthModel.Users{
		Common: oauthModel.Common{
			ID: userID,
		},
	}
}

func (caDto *UpdateUsernameRequest) ValidateUsernameDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Username,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type UsernameResponse struct {
	Status bool `json:"status"`
}


// internal/authentication/exception/auth_exception.go
package authException

import (
	errorList "github.com/diki-haryadi/go-micro-template/pkg/constant/error/error_list"
	customErrors "github.com/diki-haryadi/go-micro-template/pkg/error/custom_error"
	errorUtils "github.com/diki-haryadi/go-micro-template/pkg/error/error_utils"
)

func AuthorizationCodeGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func AuthorizationCodeGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func PasswordGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func PasswordGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func GrantClientCredentialGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func GrantClientCredentialGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func RefreshTokenGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func RefreshTokenGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func IntrospectValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func IntrospectBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}


// internal/authentication/job/job.go
package authJob

import (
	"context"

	"go.uber.org/zap"

	"github.com/robfig/cron/v3"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"

	cronJob "github.com/diki-haryadi/ztools/cron"
)

type job struct {
	cron   *cron.Cron
	logger *zap.Logger
}

func NewJob(logger *zap.Logger) oauthDomain.Job {
	newCron := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cronJob.NewLogger()),
	))
	return &job{cron: newCron, logger: logger}
}

func (j *job) StartJobs(ctx context.Context) {
	j.logArticleJob(ctx)
	go j.cron.Start()
}

func (j *job) logArticleJob(ctx context.Context) {
	worker := wrapper.BuildChain(j.logArticleWorker(),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)

	entryId, _ := j.cron.AddFunc("*/1 * * * *",
		worker.ToWorkerFunc(ctx, nil),
	)

	j.logger.Sugar().Infof("Article Job Started: %v", entryId)
}


// internal/authentication/job/worker.go
package authJob

import (
	"context"

	"github.com/diki-haryadi/ztools/wrapper"
)

func (j *job) logArticleWorker() wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		j.logger.Info("article log job")
		return nil, nil
	}
}


// internal/authentication/repository/auth_repo.go
package authRepository

import (
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
	"github.com/diki-haryadi/ztools/postgres"
)

type repository struct {
	postgres *postgres.Postgres
}

func NewRepository(conn *postgres.Postgres) oauthDomain.Repository {
	return &repository{postgres: conn}
}


// internal/authentication/repository/client_repo.go
package authRepository

import (
	"context"
	"database/sql"
	authDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (rp *repository) CreateClientCommon(ctx context.Context, clientID, secret, redirectURI string) (*authDomain.Client, error) {
	// 1. Check if client already exists
	var existingClient authDomain.Client
	sqlCheck := `SELECT id FROM clients WHERE client_id = $1`
	err := rp.postgres.SqlxDB.GetContext(ctx, &existingClient, sqlCheck, clientID)
	if err == nil {
		return nil, response.ErrClientIDTaken // Client ID is already taken
	}
	if err != sql.ErrNoRows {
		return nil, err // Other errors
	}

	// 2. Hash the secret (password)
	secretHash, err := pkg.HashPassword(secret)
	if err != nil {
		return nil, err
	}

	// 3. Insert the new client into the database
	sqlInsert := `
        INSERT INTO clients (client_id, secret, redirect_uri, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id, client_id, secret, redirect_uri, created_at
    `

	client := &authDomain.Client{
		Common: authDomain.Common{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		},
		Key:         strings.ToLower(clientID),
		Secret:      string(secretHash),
		RedirectURI: pkg.StringOrNull(redirectURI),
	}

	// Execute the insert query and scan the results into the client struct
	err = rp.postgres.SqlxDB.QueryRowContext(ctx, sqlInsert, client.Key, client.Secret, client.RedirectURI, client.CreatedAt).Scan(
		&client.ID, &client.Key, &client.Secret, &client.RedirectURI, &client.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (rp *repository) FindClientByClientID(ctx context.Context, clientID string) (*authDomain.Client, error) {
	client := authDomain.Client{}
	query := "SELECT * FROM clients WHERE key = $1"
	err := rp.postgres.SqlxDB.GetContext(ctx, &client, query, strings.ToLower(clientID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrClientNotFound
		}
		return nil, err
	}

	return &client, err
}


// internal/authentication/repository/role.go
package authRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// FindRoleByID retrieves a role by its ID using raw SQL
func (rp *repository) FindRoleByID(ctx context.Context, id string) (*oauthDomain.Role, error) {
	sqlQuery := "SELECT id, name FROM roles WHERE id = $1"

	role := new(oauthDomain.Role)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, id).Scan(&role.ID, &role.Name)

	if err == sql.ErrNoRows {
		return nil, response.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}

	return role, nil
}


// internal/authentication/repository/scope.go
package authRepository

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"sort"
	"strconv"
	"strings"
)

// GetScope takes a requested scope and, if it's empty, returns the default
// scope, if not empty, it validates the requested scope
func (rp *repository) GetScope(ctx context.Context, requestedScope string) (string, error) {
	// Return the default scope if the requested scope is empty
	if requestedScope == "" {
		return rp.GetDefaultScope(ctx), nil
	}

	// If the requested scope exists in the database, return it
	if rp.ScopeExists(ctx, requestedScope) {
		return requestedScope, nil
	}

	// Otherwise return error
	return "", response.ErrInvalidScope
}

// GetDefaultScope retrieves the default scope from the database using raw SQL
func (rp *repository) GetDefaultScope(ctx context.Context) string {
	// Fetch default scopes from the database using raw SQL
	sqlQuery := "SELECT scope FROM scopes WHERE is_default = $1"
	rows, err := rp.postgres.SqlxDB.QueryContext(ctx, sqlQuery, true)
	if err != nil {
		// Handle error (e.g., database connection issues)
		return ""
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			// Handle error (e.g., scanning issues)
			return ""
		}
		scopes = append(scopes, scope)
	}

	// Sort the scopes alphabetically
	sort.Strings(scopes)

	// Return space-delimited scope string
	return strings.Join(scopes, " ")
}

// ScopeExists checks if a scope exists using raw SQL
func (rp *repository) ScopeExists(ctx context.Context, requestedScope string) bool {
	scopes := strings.Split(requestedScope, ",")

	query := "SELECT COUNT(*) FROM scopes WHERE scope IN ("

	placeholders := make([]string, len(scopes))
	for i := range scopes {
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}
	query += strings.Join(placeholders, ", ") + ")"

	var count int
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, query, scopes).Scan(&count)
	if err != nil {
		return false
	}

	return count == len(scopes)
}


// internal/authentication/repository/user.go
package authRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (rp *repository) FindUserByUsername(ctx context.Context, username string) (*oauthDomain.Users, error) {
	sqlQuery := "SELECT id, username, password, role_id, created_at, updated_at FROM users WHERE LOWER(username) = $1"

	user := new(oauthDomain.Users)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, strings.ToLower(username)).Scan(
		&user.ID, &user.Username, &user.Password, &user.RoleID, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, response.ErrUserNotFound
	}
	if err != nil {
		return nil, err // Handle any other error
	}

	return user, nil
}

func (rp *repository) CreateUserCommon(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	user := &oauthDomain.Users{
		Common: oauthDomain.Common{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		},
		RoleID:   pkg.StringOrNull(roleID),
		Username: strings.ToLower(username),
		Password: pkg.StringOrNull(""),
	}

	// If the password is being set, hash it
	if password != "" {
		if len(password) < response.MinPasswordLength {
			return nil, response.ErrPasswordTooShort
		}

		passwordHash, err := pkg.HashPassword(password)
		if err != nil {
			return nil, err
		}
		user.Password = pkg.StringOrNull(string(passwordHash))
	}

	// Check if the username is already taken using raw SQL
	sqlCheckUsername := "SELECT COUNT(*) FROM users WHERE LOWER(username) = $1"
	var count int
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlCheckUsername, user.Username).Scan(&count)
	if err != nil {
		return nil, err
	}

	if count > 0 {
		return nil, response.ErrUsernameTaken
	}

	// Insert the new user into the database
	sqlInsert := `
        INSERT INTO users (id, created_at, role_id, username, password)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, role_id, username, password
    `
	err = rp.postgres.SqlxDB.QueryRowContext(ctx, sqlInsert, user.ID, user.CreatedAt, user.RoleID, user.Username, user.Password).
		Scan(&user.ID, &user.CreatedAt, &user.RoleID, &user.Username, &user.Password)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// SetPasswordCommon updates the user's password using raw SQL
func (rp *repository) SetPasswordCommon(ctx context.Context, user *oauthDomain.Users, password string) error {
	if len(password) < response.MinPasswordLength {
		return response.ErrPasswordTooShort
	}

	// Create a bcrypt hash for the password
	passwordHash, err := pkg.HashPassword(password)
	if err != nil {
		return err
	}

	// Prepare the SQL query to update the password and the updated_at field
	sqlQuery := `
        UPDATE users
        SET password = $1, updated_at = $2
        WHERE id = $3
    `

	// Execute the query to update the user's password
	_, err = rp.postgres.SqlxDB.ExecContext(ctx, sqlQuery, string(passwordHash), time.Now().UTC(), user.ID)
	if err != nil {
		return err
	}

	return nil
}

// UpdateUsernameCommon updates the user's username using raw SQL
func (rp *repository) UpdateUsernameCommon(ctx context.Context, user *oauthDomain.Users, username string) error {
	if username == "" {
		return response.ErrCannotSetEmptyUsername
	}

	// Prepare the SQL query to update the username field
	sqlQuery := `
        UPDATE users
        SET username = $1
        WHERE id = $2`

	// Execute the query to update the username
	_, err := rp.postgres.SqlxDB.ExecContext(ctx, sqlQuery, strings.ToLower(username), user.ID)
	if err != nil {
		return err
	}

	return nil
}

func (rp *repository) UpdatePassword(ctx context.Context, uuid, password string) error {
	if password == "" {
		return response.ErrUserPasswordNotSet
	}

	// Prepare the SQL query to update the username field
	sqlQuery := `
        UPDATE users
        SET password = $1
        WHERE id = $2`

	// Execute the query to update the username
	_, err := rp.postgres.SqlxDB.Exec(sqlQuery, password, uuid)
	if err != nil {
		return err
	}

	return nil
}

// FetchUserByUserID retrieves the user by user_id using raw SQL
func (rp *repository) FetchUserByUserID(ctx context.Context, userID string) (*oauthDomain.Users, error) {
	sqlUserQuery := "SELECT id, username, password FROM users WHERE id = $1"
	user := new(oauthDomain.Users)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlUserQuery, userID).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}


// internal/authentication/tests/fixtures/auth_integration_fixture.go
package oauthFixture

import (
	"context"
	"math"
	"net"
	"time"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	oauthGrpc "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/grpc"
	oauthHttp "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/http"
	oauthKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/authentication/delivery/kafka/producer"
	oauthRepo "github.com/diki-haryadi/go-micro-template/internal/authentication/repository"
	oauthUseCase "github.com/diki-haryadi/go-micro-template/internal/authentication/usecase"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	iContainer "github.com/diki-haryadi/ztools/infra_container"
	"github.com/diki-haryadi/ztools/logger"
)

const BUFSIZE = 1024 * 1024

type IntegrationTestFixture struct {
	TearDown          func()
	Ctx               context.Context
	Cancel            context.CancelFunc
	InfraContainer    *iContainer.IContainer
	ArticleGrpcClient articleV1.ArticleServiceClient
}

func NewIntegrationTestFixture() (*IntegrationTestFixture, error) {
	deadline := time.Now().Add(time.Duration(math.MaxInt64))
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	container := iContainer.IContainer{}
	ic, infraDown, err := container.IContext(ctx).
		ICDown().ICPostgres().ICGrpc().ICEcho().
		ICKafka().NewIC()
	if err != nil {
		cancel()
		return nil, err
	}

	extBridge, extBridgeDown, err := externalBridge.NewExternalBridge(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(extBridge.SampleExtGrpcService)
	kafkaProducer := oauthKafkaProducer.NewProducer(ic.KafkaWriter)
	repository := oauthRepo.NewRepository(ic.Postgres)
	useCase := oauthUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// http
	ic.EchoHttpServer.SetupDefaultMiddlewares()
	httpRouterGp := ic.EchoHttpServer.GetEchoInstance().Group(ic.EchoHttpServer.GetBasePath())
	httpController := oauthHttp.NewController(useCase)
	oauthHttp.NewRouter(httpController).Register(httpRouterGp)

	// grpc
	grpcController := oauthGrpc.NewController(useCase)
	articleV1.RegisterArticleServiceServer(ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	lis := bufconn.Listen(BUFSIZE)
	go func() {
		if err := ic.GrpcServer.GetCurrentGrpcServer().Serve(lis); err != nil {
			logger.Zap.Sugar().Fatalf("Server exited with error: %v", err)
		}
	}()

	grpcClientConn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	articleGrpcClient := articleV1.NewArticleServiceClient(grpcClientConn)

	return &IntegrationTestFixture{
		TearDown: func() {
			cancel()
			infraDown()
			_ = grpcClientConn.Close()
			extBridgeDown()
		},
		InfraContainer:    ic,
		Ctx:               ctx,
		Cancel:            cancel,
		ArticleGrpcClient: articleGrpcClient,
	}, nil
}


// internal/authentication/tests/integrations/create_auth_test.go
package artcileIntegrationTest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"github.com/labstack/echo/v4"

	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	articleFixture "github.com/diki-haryadi/go-micro-template/internal/article/tests/fixtures"
	grpcError "github.com/diki-haryadi/ztools/error/grpc"
	httpError "github.com/diki-haryadi/ztools/error/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type testSuite struct {
	suite.Suite
	fixture *articleFixture.IntegrationTestFixture
}

func (suite *testSuite) SetupSuite() {
	fixture, err := articleFixture.NewIntegrationTestFixture()
	if err != nil {
		assert.Error(suite.T(), err)
	}

	suite.fixture = fixture
}

func (suite *testSuite) TearDownSuite() {
	suite.fixture.TearDown()
}

func (suite *testSuite) TestSuccessfulCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro Developer",
	}

	response, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)
	if err != nil {
		assert.Error(suite.T(), err)
	}

	assert.NotNil(suite.T(), response.Id)
	assert.Equal(suite.T(), "John", response.Name)
	assert.Equal(suite.T(), "Pro Developer", response.Desc)
}

func (suite *testSuite) TestNameValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "Jo",
		Desc: "Pro Developer",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "name")
}

func (suite *testSuite) TestDescValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "desc")
}

func (suite *testSuite) TestSuccessCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusOK, response.Code)

	caDto := new(articleDto.CreateArticleRequestDto)
	if assert.NoError(suite.T(), json.Unmarshal(response.Body.Bytes(), caDto)) {
		assert.Equal(suite.T(), "John Snow", caDto.Name)
		assert.Equal(suite.T(), "King of the north", caDto.Description)
	}

}

func (suite *testSuite) TestNameValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"Jo","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "name")
	}

}

func (suite *testSuite) TestDescValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()

	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "desc")
	}
}

func TestRunSuite(t *testing.T) {
	tSuite := new(testSuite)
	suite.Run(t, tSuite)
}


// internal/authentication/usecase/auth_usecase.go
package authUseCase

import (
	sampleExtServiceDomain "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/domain"
	authDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain"
)

type useCase struct {
	repository              authDomain.Repository
	kafkaProducer           authDomain.KafkaProducer
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase
	allowedRoles            []string
}

func NewUseCase(
	repository authDomain.Repository,
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase,
	kafkaProducer authDomain.KafkaProducer,
) authDomain.UseCase {
	return &useCase{
		repository:              repository,
		kafkaProducer:           kafkaProducer,
		sampleExtServiceUseCase: sampleExtServiceUseCase,
		allowedRoles:            []string{Superuser, User},
	}
}


// internal/authentication/usecase/change_password.go
package authUseCase

import (
	"context"
	authDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) ChangePassword(ctx context.Context, dto *authDto.ChangePasswordRequest) (*authDto.ChangePasswordResponse, error) {
	if len(dto.Password) < response.MinPasswordLength {
		return &authDto.ChangePasswordResponse{}, response.ErrPasswordTooShort
	}

	user, err := uc.repository.FetchUserByUserID(ctx, dto.UUID)
	if err != nil {
		return &authDto.ChangePasswordResponse{}, nil
	}

	err = pkg.VerifyPassword(user.Password.String, dto.Password)
	if err != nil {
		return &authDto.ChangePasswordResponse{}, response.ErrInvalidPassword
	}

	passwordHash, err := pkg.HashPassword(dto.NewPassword)
	if err != nil {
		return &authDto.ChangePasswordResponse{}, err
	}

	err = uc.repository.UpdatePassword(ctx, dto.UUID, string(passwordHash))
	if err != nil {
		return &authDto.ChangePasswordResponse{}, nil
	}

	return &authDto.ChangePasswordResponse{
		Status: true,
	}, nil
}


// internal/authentication/usecase/client_usecase.go
package authUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) AuthClient(ctx context.Context, clientID, secret string) (*authDomain.Client, error) {
	client, err := uc.repository.FindClientByClientID(ctx, clientID)
	if err != nil {
		return nil, response.ErrClientNotFound
	}

	if pkg.VerifyPassword(client.Secret, secret) != nil {
		return nil, response.ErrInvalidClientSecret
	}
	return client, nil
}

func (uc *useCase) CreateClient(ctx context.Context, clientID, secret, redirectURI string) (*authDomain.Client, error) {
	client, err := uc.repository.CreateClientCommon(ctx, clientID, secret, redirectURI)
	if err != nil {
		return nil, err
	}
	return client, err
}

func (uc *useCase) ClientExists(ctx context.Context, clientID string) bool {
	_, err := uc.repository.FindClientByClientID(ctx, clientID)
	return err == nil
}


// internal/authentication/usecase/forgot_password.go
package authUseCase

import (
	"context"
	authDto "github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) ForgotPassword(ctx context.Context, dto *authDto.ForgotPasswordRequest) (*authDto.ForgotPasswordResponse, error) {
	if len(dto.Password) < response.MinPasswordLength {
		return &authDto.ForgotPasswordResponse{}, response.ErrPasswordTooShort
	}

	user, err := uc.repository.FetchUserByUserID(ctx, dto.UUID)
	if err != nil {
		return &authDto.ForgotPasswordResponse{}, nil
	}

	err = pkg.VerifyPassword(user.Password.String, dto.Password)
	if err == nil {
		return &authDto.ForgotPasswordResponse{}, response.ErrInvalidPasswordCannotSame
	}

	passwordHash, err := pkg.HashPassword(dto.Password)
	if err != nil {
		return &authDto.ForgotPasswordResponse{}, err
	}

	err = uc.repository.UpdatePassword(ctx, dto.UUID, string(passwordHash))
	if err != nil {
		return &authDto.ForgotPasswordResponse{}, nil
	}

	return &authDto.ForgotPasswordResponse{
		Status: true,
	}, nil
}


// internal/authentication/usecase/register.go
package authUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/internal/authentication/dto"
)

func (uc *useCase) Register(ctx context.Context, dto *authDto.UserRequestDto) (*authDto.UserResponse, error) {
	user, err := uc.repository.CreateUserCommon(ctx, dto.RoleID, dto.Username, dto.Password)
	if err != nil {
		return &authDto.UserResponse{}, nil
	}
	return &authDto.UserResponse{
		UUID:     user.ID.String(),
		Username: user.Username,
		RoleID:   user.RoleID.String,
	}, nil
}


// internal/authentication/usecase/roles.go
package authUseCase

import (
	"errors"
	"strings"
)

const (
	// Superuser ...
	Superuser = "superuser"
	// User ...
	User = "user"
)

var roleWeight = map[string]int{
	Superuser: 100,
	User:      1,
}

// IsGreaterThan returns true if role1 is greater than role2
func (uc *useCase) IsGreaterThan(role1, role2 string) (bool, error) {
	// Get weight of the first role
	weight1, ok := roleWeight[role1]
	if !ok {
		return false, errors.New("Role weight not found")
	}

	// Get weight of the second role
	weight2, ok := roleWeight[role2]
	if !ok {
		return false, errors.New("Role weight not found")
	}

	return weight1 > weight2, nil
}

// RestrictToRoles restricts this service to only specified roles
func (uc *useCase) RestrictToRoles(allowedRoles ...string) {
	uc.allowedRoles = allowedRoles
}

// IsRoleAllowed returns true if the role is allowed to use this service
func (uc *useCase) IsRoleAllowed(role string) bool {
	for _, allowedRole := range uc.allowedRoles {
		if strings.ToLower(role) == allowedRole {
			return true
		}
	}
	return false
}


// internal/authentication/usecase/scope_usecase.go
package authUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) GetScope(ctx context.Context, requestScope string) (string, error) {
	if requestScope == "" {
		scope := uc.repository.GetDefaultScope(ctx)
		return scope, nil
	}

	if scope := uc.repository.ScopeExists(ctx, requestScope); scope {
		return requestScope, nil
	}
	return "", response.ErrInvalidScope
}


// internal/authentication/usecase/users_usecase.go
package authUseCase

import (
	"context"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/authentication/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// UserExists returns true if user exists
func (uc *useCase) UserExists(ctx context.Context, username string) bool {
	_, err := uc.repository.FindUserByUsername(ctx, username)
	return err == nil
}

// CreateUser saves a new user to database
func (uc *useCase) CreateUser(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	return uc.repository.CreateUserCommon(ctx, roleID, username, password)
}

// CreateUserTx saves a new user to database using injected db object
func (uc *useCase) CreateUserTx(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	return uc.repository.CreateUserCommon(ctx, roleID, username, password)
}

// SetPassword sets a user password
func (uc *useCase) SetPassword(ctx context.Context, user *oauthDomain.Users, password string) error {
	return uc.repository.SetPasswordCommon(ctx, user, password)
}

// SetPasswordTx sets a user password in a transaction
func (uc *useCase) SetPasswordTx(ctx context.Context, user *oauthDomain.Users, password string) error {
	return uc.repository.SetPasswordCommon(ctx, user, password)
}

// AuthUser authenticates user
func (uc *useCase) AuthUser(ctx context.Context, username, password string) (*oauthDomain.Users, error) {
	// Fetch the user
	user, err := uc.repository.FindUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// Check that the password is set
	if !user.Password.Valid {
		return nil, err
	}

	// Verify the password
	if pkg.VerifyPassword(user.Password.String, password) != nil {
		return nil, err
	}

	role, err := uc.repository.FindRoleByID(ctx, user.RoleID.String)
	if err != nil {
		return nil, err
	}
	user.Role = role
	return user, nil
}

// UpdateUsername ...
func (uc *useCase) UpdateUsername(ctx context.Context, user *oauthDomain.Users, username string) error {
	if username == "" {
		return response.ErrCannotSetEmptyUsername
	}

	return uc.repository.UpdateUsernameCommon(ctx, user, username)
}

// UpdateUsernameTx ...
func (uc *useCase) UpdateUsernameTx(ctx context.Context, user *oauthDomain.Users, username string) error {
	return uc.repository.UpdateUsernameCommon(ctx, user, username)
}


// internal/health_check/configurator/health_check_configurator.go
package healthCheckConfigurator

import (
	"context"

	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"

	kafkaHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/kafka_health_check"
	postgresHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/postgres_health_check"
	tmpDirHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/tmp_dir_health_check"

	healthCheckGrpc "github.com/diki-haryadi/go-micro-template/internal/health_check/delivery/grpc"
	healthCheckHttp "github.com/diki-haryadi/go-micro-template/internal/health_check/delivery/http"
	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
	healthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase"
	infraContainer "github.com/diki-haryadi/ztools/infra_container"
)

type configurator struct {
	ic *infraContainer.IContainer
}

func NewConfigurator(ic *infraContainer.IContainer) healthCheckDomain.Configurator {
	return &configurator{ic: ic}
}

func (c *configurator) Configure(ctx context.Context) error {
	postgresHealthCheckUc := postgresHealthCheckUseCase.NewUseCase(c.ic.Postgres)
	kafkaHealthCheckUc := kafkaHealthCheckUseCase.NewUseCase()
	tmpDirHealthCheckUc := tmpDirHealthCheckUseCase.NewUseCase()

	healthCheckUc := healthCheckUseCase.NewUseCase(postgresHealthCheckUc, kafkaHealthCheckUc, tmpDirHealthCheckUc)

	// grpc
	grpcController := healthCheckGrpc.NewController(healthCheckUc, postgresHealthCheckUc, kafkaHealthCheckUc, tmpDirHealthCheckUc)
	grpcHealthV1.RegisterHealthServer(c.ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	// http
	httpRouterGp := c.ic.EchoHttpServer.GetEchoInstance().Group(c.ic.EchoHttpServer.GetBasePath())
	httpController := healthCheckHttp.NewController(healthCheckUc)
	healthCheckHttp.NewRouter(httpController).Register(httpRouterGp)

	return nil
}


// internal/health_check/delivery/grpc/health_check_grpc_controller.go
package healthCheckGrpc

import (
	"context"

	"google.golang.org/grpc/codes"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
)

type controller struct {
	healthCheckUseCase    healthCheckDomain.HealthCheckUseCase
	postgresHealthCheckUc healthCheckDomain.PostgresHealthCheckUseCase
	kafkaHealthCheckUc    healthCheckDomain.KafkaHealthCheckUseCase
	tmpDirHealthCheckUc   healthCheckDomain.TmpDirHealthCheckUseCase
}

func NewController(
	healthCheckUc healthCheckDomain.HealthCheckUseCase,
	postgresHealthCheckUc healthCheckDomain.PostgresHealthCheckUseCase,
	kafkaHealthCheckUc healthCheckDomain.KafkaHealthCheckUseCase,
	tmpDirHealthCheckUc healthCheckDomain.TmpDirHealthCheckUseCase) healthCheckDomain.GrpcController {
	return &controller{
		healthCheckUseCase:    healthCheckUc,
		postgresHealthCheckUc: postgresHealthCheckUc,
		kafkaHealthCheckUc:    kafkaHealthCheckUc,
		tmpDirHealthCheckUc:   tmpDirHealthCheckUc,
	}
}

func (c *controller) Check(ctx context.Context, request *grpcHealthV1.HealthCheckRequest) (*grpcHealthV1.HealthCheckResponse, error) {
	var healthStatus bool

	switch request.Service {
	case "", "all":
		healthStatus = c.healthCheckUseCase.Check().Status
	case "kafka":
		healthStatus = c.kafkaHealthCheckUc.Check()
	case "postgres":
		healthStatus = c.postgresHealthCheckUc.Check()
	case "writable-tmp-dir":
		healthStatus = c.tmpDirHealthCheckUc.Check()
	default:
		return &grpcHealthV1.HealthCheckResponse{
			Status: grpcHealthV1.HealthCheckResponse_UNKNOWN,
		}, nil
	}

	grpcStatus := grpcHealthV1.HealthCheckResponse_SERVING

	if !healthStatus {
		grpcStatus = grpcHealthV1.HealthCheckResponse_NOT_SERVING
	}

	return &grpcHealthV1.HealthCheckResponse{
		Status: grpcStatus,
	}, nil
}

func (c *controller) Watch(request *grpcHealthV1.HealthCheckRequest, server grpcHealthV1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "unimplemented")
}


// internal/health_check/delivery/http/health_check_http_controller.go
package healthCheckHttp

import (
	"net/http"

	"github.com/labstack/echo/v4"

	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
)

type controller struct {
	useCase healthCheckDomain.HealthCheckUseCase
}

func NewController(useCase healthCheckDomain.HealthCheckUseCase) healthCheckDomain.HttpController {
	return &controller{
		useCase: useCase,
	}
}

func (c controller) Check(eCtx echo.Context) error {
	healthResult := c.useCase.Check()

	httpStatus := http.StatusOK
	if !healthResult.Status {
		httpStatus = http.StatusInternalServerError
	}

	return eCtx.JSON(httpStatus, healthResult)
}


// internal/health_check/delivery/http/health_check_http_router.go
package healthCheckHttp

import (
	"github.com/labstack/echo/v4"

	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
)

type Router struct {
	controller healthCheckDomain.HttpController
}

func NewRouter(controller healthCheckDomain.HttpController) *Router {
	return &Router{
		controller: controller,
	}
}

func (r *Router) Register(e *echo.Group) {
	e.GET("/health", r.controller.Check)
}


// internal/health_check/domain/health_check_domain.go
package healthCheckDomain

import (
	"context"

	"github.com/labstack/echo/v4"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"

	healthCheckDto "github.com/diki-haryadi/go-micro-template/internal/health_check/dto"
)

type Configurator interface {
	Configure(ctx context.Context) error
}

type GrpcController interface {
	Check(ctx context.Context, request *grpcHealthV1.HealthCheckRequest) (*grpcHealthV1.HealthCheckResponse, error)
	Watch(request *grpcHealthV1.HealthCheckRequest, server grpcHealthV1.Health_WatchServer) error
}

type HttpController interface {
	Check(c echo.Context) error
}

type HealthCheckUseCase interface {
	Check() *healthCheckDto.HealthCheckResponseDto
}

type PostgresHealthCheckUseCase interface {
	Check() bool
}

type TmpDirHealthCheckUseCase interface {
	Check() bool
}

type KafkaHealthCheckUseCase interface {
	Check() bool
}


// internal/health_check/dto/health_check_dto.go
package healthCheckDto

type HealthCheckUnit struct {
	Unit string `json:"unit"`
	Up   bool   `json:"up"`
}

type HealthCheckResponseDto struct {
	Status bool              `json:"status"`
	Units  []HealthCheckUnit `json:"units"`
}


// internal/health_check/tests/fixtures/health_check_integration_fixture.go
package healthCheckFixture

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	healthCheckGrpc "github.com/diki-haryadi/go-micro-template/internal/health_check/delivery/grpc"
	healthCheckHttp "github.com/diki-haryadi/go-micro-template/internal/health_check/delivery/http"
	healthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase"
	kafkaHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/kafka_health_check"
	postgresHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/postgres_health_check"
	tmpDirHealthCheckUseCase "github.com/diki-haryadi/go-micro-template/internal/health_check/usecase/tmp_dir_health_check"
	"github.com/diki-haryadi/ztools/logger"

	iContainer "github.com/diki-haryadi/ztools/infra_container"
)

type IntegrationTestFixture struct {
	TearDown              func()
	Ctx                   context.Context
	Cancel                context.CancelFunc
	InfraContainer        *iContainer.IContainer
	HealthCheckGrpcClient grpcHealthV1.HealthClient
}

const BUFSIZE = 1024 * 1024

func NewIntegrationTestFixture() (*IntegrationTestFixture, error) {
	deadline := time.Now().Add(time.Duration(1 * time.Minute))
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	ic, infraDown, err := iContainer.NewIC(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	postgresHealthCheckUc := postgresHealthCheckUseCase.NewUseCase(ic.Postgres)
	kafkaHealthCheckUc := kafkaHealthCheckUseCase.NewUseCase()
	tmpDirHealthCheckUc := tmpDirHealthCheckUseCase.NewUseCase()

	healthCheckUc := healthCheckUseCase.NewUseCase(postgresHealthCheckUc, kafkaHealthCheckUc, tmpDirHealthCheckUc)

	// http
	ic.EchoHttpServer.SetupDefaultMiddlewares()
	httpRouterGp := ic.EchoHttpServer.GetEchoInstance().Group(ic.EchoHttpServer.GetBasePath())
	httpController := healthCheckHttp.NewController(healthCheckUc)
	healthCheckHttp.NewRouter(httpController).Register(httpRouterGp)

	// grpc
	grpcController := healthCheckGrpc.NewController(healthCheckUc, postgresHealthCheckUc, kafkaHealthCheckUc, tmpDirHealthCheckUc)
	grpcHealthV1.RegisterHealthServer(ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	lis := bufconn.Listen(BUFSIZE)
	go func() {
		if err := ic.GrpcServer.GetCurrentGrpcServer().Serve(lis); err != nil {
			logger.Zap.Sugar().Fatalf("Server exited with error: %v", err)
		}
	}()

	grpcClientConn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	healthCheckGrpcClient := grpcHealthV1.NewHealthClient(grpcClientConn)

	return &IntegrationTestFixture{
		TearDown: func() {
			cancel()
			infraDown()
			_ = grpcClientConn.Close()
		},
		InfraContainer:        ic,
		Ctx:                   ctx,
		Cancel:                cancel,
		HealthCheckGrpcClient: healthCheckGrpcClient,
	}, nil
}


// internal/health_check/tests/integrations/health_check_test.go
package artcileIntegrationTest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"

	healthCheckDto "github.com/diki-haryadi/go-micro-template/internal/health_check/dto"
	healthCheckFixture "github.com/diki-haryadi/go-micro-template/internal/health_check/tests/fixtures"
)

type testSuite struct {
	suite.Suite
	fixture *healthCheckFixture.IntegrationTestFixture
}

func (suite *testSuite) SetupSuite() {
	fixture, err := healthCheckFixture.NewIntegrationTestFixture()
	if err != nil {
		assert.Error(suite.T(), err)
	}

	suite.fixture = fixture
}

func (suite *testSuite) TearDownSuite() {
	suite.fixture.TearDown()
}

func (suite *testSuite) TestHealthCheckHttpShouldSendOkForAllUnits() {

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusOK, response.Code)

	healthCheckResponseDto := new(healthCheckDto.HealthCheckResponseDto)

	if assert.NoError(suite.T(), json.Unmarshal(response.Body.Bytes(), healthCheckResponseDto)) {
		assert.Equal(suite.T(), true, healthCheckResponseDto.Status)
		assert.Equal(suite.T(), []healthCheckDto.HealthCheckUnit{
			{
				Unit: "postgres",
				Up:   true,
			},
			{
				Unit: "kafka",
				Up:   true,
			},
			{
				Unit: "writable-tmp-dir",
				Up:   true,
			},
		}, healthCheckResponseDto.Units)
	}

}

func (suite *testSuite) TestHealthCheckGrpcShouldSendServingForAllServices() {
	ctx := context.Background()

	healthCheckRequest := &grpcHealthV1.HealthCheckRequest{
		Service: "all",
	}
	response, _ := suite.fixture.HealthCheckGrpcClient.Check(ctx, healthCheckRequest)

	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), grpcHealthV1.HealthCheckResponse_SERVING, response.GetStatus())
}

func (suite *testSuite) TestHealthCheckGrpcShouldSendUnknownForUnknownService() {
	ctx := context.Background()

	healthCheckRequest := &grpcHealthV1.HealthCheckRequest{
		Service: "un-known-service",
	}
	response, _ := suite.fixture.HealthCheckGrpcClient.Check(ctx, healthCheckRequest)

	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), grpcHealthV1.HealthCheckResponse_UNKNOWN, response.GetStatus())
}

func TestRunSuite(t *testing.T) {
	tSuite := new(testSuite)
	suite.Run(t, tSuite)
}


// internal/health_check/usecase/health_check_usecase.go
package healthCheckUseCase

import (
	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
	healthCheckDto "github.com/diki-haryadi/go-micro-template/internal/health_check/dto"
)

type useCase struct {
	postgresHealthCheckUc healthCheckDomain.PostgresHealthCheckUseCase
	kafkaHealthCheckUc    healthCheckDomain.KafkaHealthCheckUseCase
	tmpDirHealthCheckUc   healthCheckDomain.TmpDirHealthCheckUseCase
}

func NewUseCase(
	postgresHealthCheckUc healthCheckDomain.PostgresHealthCheckUseCase,
	kafkaHealthCheckUc healthCheckDomain.KafkaHealthCheckUseCase,
	tmpDirHealthCheckUc healthCheckDomain.TmpDirHealthCheckUseCase,
) healthCheckDomain.HealthCheckUseCase {
	return &useCase{
		postgresHealthCheckUc: postgresHealthCheckUc,
		kafkaHealthCheckUc:    kafkaHealthCheckUc,
		tmpDirHealthCheckUc:   tmpDirHealthCheckUc,
	}
}

func (uc *useCase) Check() *healthCheckDto.HealthCheckResponseDto {
	healthCheckResult := healthCheckDto.HealthCheckResponseDto{
		Status: true,
		Units:  nil,
	}

	pgUnit := healthCheckDto.HealthCheckUnit{
		Unit: "postgres",
		Up:   uc.postgresHealthCheckUc.Check(),
	}
	healthCheckResult.Units = append(healthCheckResult.Units, pgUnit)

	kafkaUnit := healthCheckDto.HealthCheckUnit{
		Unit: "kafka",
		Up:   uc.kafkaHealthCheckUc.Check(),
	}
	healthCheckResult.Units = append(healthCheckResult.Units, kafkaUnit)

	tmpDirUnit := healthCheckDto.HealthCheckUnit{
		Unit: "writable-tmp-dir",
		Up:   uc.tmpDirHealthCheckUc.Check(),
	}
	healthCheckResult.Units = append(healthCheckResult.Units, tmpDirUnit)

	for _, v := range healthCheckResult.Units {
		if !v.Up {
			healthCheckResult.Status = false
			break
		}
	}

	return &healthCheckResult
}


// internal/health_check/usecase/kafka_health_check/kafka_health_check_usecase.go
package kafkaHealthCheckUseCase

import (
	"github.com/segmentio/kafka-go"

	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
	"github.com/diki-haryadi/ztools/config"
)

type useCase struct{}

func NewUseCase() healthCheckDomain.KafkaHealthCheckUseCase {
	return &useCase{}
}

func (uc *useCase) Check() bool {
	brokers := kafka.TCP(config.BaseConfig.Kafka.ClientBrokers...)

	conn, err := kafka.Dial(brokers.Network(), brokers.String())
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}


// internal/health_check/usecase/postgres_health_check/postgres_health_check_usecase.go
package postgresHealthCheckUseCase

import (
	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
	"github.com/diki-haryadi/ztools/postgres"
)

type useCase struct {
	postgres *postgres.Postgres
}

func NewUseCase(postgres *postgres.Postgres) healthCheckDomain.PostgresHealthCheckUseCase {
	return &useCase{
		postgres: postgres,
	}
}

func (uc *useCase) Check() bool {
	if err := uc.postgres.SqlxDB.DB.Ping(); err != nil {
		return false
	}
	return true
}


// internal/health_check/usecase/tmp_dir_health_check/tmp_dir_health_check_usecase.go
package tmpDirHealthCheckUseCase

import (
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"

	healthCheckDomain "github.com/diki-haryadi/go-micro-template/internal/health_check/domain"
	"github.com/diki-haryadi/ztools/config"
)

type useCase struct{}

func NewUseCase() healthCheckDomain.TmpDirHealthCheckUseCase {
	return &useCase{}
}

func (uc *useCase) Check() bool {
	if !config.IsProdEnv() {
		return true
	}

	_, callerDir, _, ok := runtime.Caller(0)
	if !ok {
		return false
	}

	tmpDir := filepath.Join(filepath.Dir(callerDir), "../../../..", "tmp")

	return unix.Access(tmpDir, unix.W_OK) == nil
}


// internal/oauth/configurator/oauth_configurator.go
package oauthConfigurator

import (
	"context"
	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	oauthGrpcController "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/grpc"
	oauthHttpController "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/http"
	oauthKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/kafka/producer"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	oauthRepository "github.com/diki-haryadi/go-micro-template/internal/oauth/repository"
	oauthUseCase "github.com/diki-haryadi/go-micro-template/internal/oauth/usecase"
	oauth2 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/oauth2/v1"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	infraContainer "github.com/diki-haryadi/ztools/infra_container"
)

type configurator struct {
	ic        *infraContainer.IContainer
	extBridge *externalBridge.ExternalBridge
}

func NewConfigurator(ic *infraContainer.IContainer, extBridge *externalBridge.ExternalBridge) oauthDomain.Configurator {
	return &configurator{ic: ic, extBridge: extBridge}
}

func (c *configurator) Configure(ctx context.Context) error {
	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(c.extBridge.SampleExtGrpcService)
	kafkaProducer := oauthKafkaProducer.NewProducer(c.ic.KafkaWriter)
	repository := oauthRepository.NewRepository(c.ic.Postgres)
	useCase := oauthUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// grpc
	grpcController := oauthGrpcController.NewController(useCase)
	oauth2.RegisterOauth2ServiceServer(c.ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	// http
	httpRouterGp := c.ic.EchoHttpServer.GetEchoInstance().Group(c.ic.EchoHttpServer.GetBasePath())
	httpController := oauthHttpController.NewController(useCase)
	oauthHttpController.NewRouter(httpController).Register(httpRouterGp)

	// consumers
	//oauthKafkaConsumer.NewConsumer(c.ic.KafkaReader).RunConsumers(ctx)

	// jobs
	//oauthJob.NewJob(c.ic.Logger).StartJobs(ctx)

	return nil
}


// internal/oauth/delivery/grpc/oauth_grpc_controller.go
package oauthGrpcController

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	oauth2 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/oauth2/v1"
)

type controller struct {
	useCase oauthDomain.UseCase
}

func NewController(uc oauthDomain.UseCase) oauthDomain.GrpcController {
	return &controller{
		useCase: uc,
	}
}

func (c *controller) BasicAuthClient(ctx context.Context, clientID, secret string) (*oauthModel.Client, error) {

	// Authenticate the client
	client, err := c.useCase.AuthClient(ctx, clientID, secret)
	if err != nil {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	return client, nil
}

func (c *controller) PasswordGrant(ctx context.Context, req *oauth2.PasswordGrantRequest) (*oauth2.PasswordGrantResponse, error) {
	aDto := new(oauthDto.PasswordGrantRequestDto).GetFieldsValue(req.Username, req.Password, req.Scope)
	if err := aDto.ValidatePasswordDto(); err != nil {
		return &oauth2.PasswordGrantResponse{}, err
	}

	client, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &oauth2.PasswordGrantResponse{}, err
	}

	acGrant, err := c.useCase.PasswordGrant(
		ctx,
		aDto.Username,
		aDto.Password,
		aDto.Scope,
		aDto.ToModel(client.ID))

	if err != nil {
		return &oauth2.PasswordGrantResponse{}, err
	}

	return &oauth2.PasswordGrantResponse{
		AccessToken:  acGrant.AccessToken,
		TokenType:    acGrant.TokenType,
		ExpiresIn:    int32(acGrant.ExpiresIn),
		Scope:        acGrant.Scope,
		RefreshToken: acGrant.RefreshToken,
	}, nil
}

func (c *controller) AuthorizationCodeGrant(ctx context.Context, req *oauth2.AuthorizationCodeGrantRequest) (*oauth2.AuthorizationCodeGrantResponse, error) {
	aDto := new(oauthDto.AuthorizationCodeGrantRequestDto).GetFieldsValue(req.Code, req.RedirectUri, req.ClientId)
	if err := aDto.ValidateAuthorizationCodeDto(); err != nil {
		return &oauth2.AuthorizationCodeGrantResponse{}, err
	}

	client, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &oauth2.AuthorizationCodeGrantResponse{}, err
	}

	acGrant, err := c.useCase.AuthorizationCodeGrant(
		ctx,
		aDto.Code,
		aDto.RedirectUri,
		aDto.ToModel(client.ID))

	if err != nil {
		return &oauth2.AuthorizationCodeGrantResponse{}, err
	}
	return &oauth2.AuthorizationCodeGrantResponse{
		AccessToken:  acGrant.AccessToken,
		TokenType:    acGrant.TokenType,
		ExpiresIn:    int32(acGrant.ExpiresIn),
		Scope:        acGrant.Scope,
		RefreshToken: acGrant.RefreshToken,
	}, nil
}

func (c *controller) ClientCredentialsGrant(ctx context.Context, req *oauth2.ClientCredentialsGrantRequest) (*oauth2.ClientCredentialsGrantResponse, error) {
	aDto := new(oauthDto.ClientCredentialsGrantRequestDto).GetFieldsValue(req.Scope)
	if err := aDto.ValidateClientCredentialsDto(); err != nil {
		return &oauth2.ClientCredentialsGrantResponse{}, err
	}

	client, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &oauth2.ClientCredentialsGrantResponse{}, err
	}

	acGrant, err := c.useCase.GrantAccessToken(
		ctx,
		client,
		nil,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime,
		aDto.Scope)

	if err != nil {
		return &oauth2.ClientCredentialsGrantResponse{}, err
	}

	return &oauth2.ClientCredentialsGrantResponse{
		AccessToken:  acGrant.AccessToken,
		TokenType:    acGrant.TokenType,
		ExpiresIn:    int32(acGrant.ExpiresIn),
		Scope:        acGrant.Scope,
		RefreshToken: acGrant.RefreshToken,
	}, err
}

func (c *controller) RefreshTokenGrant(ctx context.Context, req *oauth2.RefreshTokenGrantRequest) (*oauth2.RefreshTokenGrantResponse, error) {
	aDto := new(oauthDto.RefreshTokenRequestDto).GetFieldsValue(req.RefreshToken, req.Scope)
	if err := aDto.ValidateRefreshTokenDto(); err != nil {
		return &oauth2.RefreshTokenGrantResponse{}, nil
	}

	client, err := c.BasicAuthClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return &oauth2.RefreshTokenGrantResponse{}, nil
	}

	acGrant, err := c.useCase.ClientCredentialsGrant(
		ctx,
		aDto.RefreshToken,
		aDto.Scope,
		aDto.ToModel(client.ID))

	if err != nil {
		return &oauth2.RefreshTokenGrantResponse{}, nil
	}
	return &oauth2.RefreshTokenGrantResponse{
		AccessToken:  acGrant.AccessToken,
		TokenType:    acGrant.TokenType,
		ExpiresIn:    int32(acGrant.ExpiresIn),
		Scope:        acGrant.Scope,
		RefreshToken: acGrant.RefreshToken,
	}, nil
}


// internal/oauth/delivery/http/oauth_http_controller.go
package oauthHttpController

import (
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type controller struct {
	useCase oauthDomain.UseCase
}

func NewController(uc oauthDomain.UseCase) oauthDomain.HttpController {
	return &controller{
		useCase: uc,
	}
}

func (c controller) Tokens(ctx echo.Context) error {
	res := response.NewJSONResponse()
	grantTypes := map[string]func(ctx echo.Context) error{
		"authorization_code": c.AuthorizationCodeGrant,
		"password":           c.PasswordGrant,
		"client_credentials": c.ClientCredentialsGrant,
		"refresh_token":      c.RefreshTokenGrant,
	}

	// Check the grant type
	grantHandler, ok := grantTypes[ctx.Request().FormValue("grant_type")]
	if !ok {
		res.SetError(response.ErrInvalidGrantType).SetMessage(response.ErrInvalidGrantType.Error()).Send(ctx.Response().Writer)
		return nil
	}

	// Grant processing
	err := grantHandler(ctx)
	if err != nil {
		res.SetError(response.ErrBadRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	return nil
}

func (c controller) AuthorizationCodeGrant(ctx echo.Context) error {
	res := response.NewJSONResponse()

	aDto := new(oauthDto.AuthorizationCodeGrantRequestDto).GetFields(ctx)
	if err := aDto.ValidateAuthorizationCodeDto(); err != nil {
		res.SetError(response.ErrInvalidAuthorizationCodeGrantRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	client, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).Send(ctx.Response().Writer)
		return nil
	}

	acGrant, err := c.useCase.AuthorizationCodeGrant(
		ctx.Request().Context(),
		aDto.Code,
		aDto.RedirectUri,
		aDto.ToModel(client.ID))

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*acGrant).Send(ctx.Response().Writer)
	return nil
}

func (c controller) PasswordGrant(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.PasswordGrantRequestDto).GetFields(ctx)
	if err := aDto.ValidatePasswordDto(); err != nil {
		res.SetError(response.ErrInvalidPasswordGrantRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	client, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	acGrant, err := c.useCase.PasswordGrant(
		ctx.Request().Context(),
		aDto.Username,
		aDto.Password,
		aDto.Scope,
		aDto.ToModel(client.ID))

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*acGrant).Send(ctx.Response().Writer)
	return nil
}

func (c controller) ClientCredentialsGrant(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.ClientCredentialsGrantRequestDto).GetFields(ctx)
	if err := aDto.ValidateClientCredentialsDto(); err != nil {
		res.SetError(response.ErrInvalidClientCredentialsGrantRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	client, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	acGrant, err := c.useCase.GrantAccessToken(
		ctx.Request().Context(),
		client,
		nil,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime,
		aDto.Scope)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(&acGrant).Send(ctx.Response().Writer)
	return nil
}

func (c controller) RefreshTokenGrant(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.RefreshTokenRequestDto).GetFields(ctx)
	if err := aDto.ValidateRefreshTokenDto(); err != nil {
		res.SetError(response.ErrInvalidClientCredentialsGrantRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	client, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	acGrant, err := c.useCase.ClientCredentialsGrant(
		ctx.Request().Context(),
		aDto.RefreshToken,
		aDto.Scope,
		aDto.ToModel(client.ID))

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*acGrant).Send(ctx.Response().Writer)
	return nil
}

func (c controller) Introspect(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.IntrospectRequestDto).GetFields(ctx)
	if err := aDto.ValidateIntrospectDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	client, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	introspect, err := c.useCase.IntrospectToken(
		ctx.Request().Context(),
		aDto.Token,
		aDto.TokenTypeHint,
		aDto.ToModel(client.ID))

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*introspect).Send(ctx.Response().Writer)
	return nil
}

func (c controller) BasicAuthClient(ctx echo.Context) (*oauthModel.Client, error) {
	// Get client credentials from basic auth
	clientID, secret, ok := ctx.Request().BasicAuth()
	if !ok {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	// Authenticate the client
	client, err := c.useCase.AuthClient(ctx.Request().Context(), clientID, secret)
	if err != nil {
		return nil, response.ErrInvalidClientIDOrSecret
	}

	return client, nil
}

func (c controller) Register(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.UserRequestDto).GetFieldsUser(ctx)
	if err := aDto.ValidateUserDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	register, err := c.useCase.Register(
		ctx.Request().Context(),
		aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*register).Send(ctx.Response().Writer)
	return nil
}

func (c controller) ChangePassword(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.ChangePasswordRequest).GetFieldsChangePassword(ctx)
	if err := aDto.ValidateChangePasswordDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	changePass, err := c.useCase.ChangePassword(
		ctx.Request().Context(), aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*changePass).Send(ctx.Response().Writer)
	return nil
}

func (c controller) ForgotPassword(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.ForgotPasswordRequest).GetFieldsForgotPassword(ctx)
	if err := aDto.ValidateForgotPasswordDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	forgotPass, err := c.useCase.ForgotPassword(
		ctx.Request().Context(), aDto)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().SetData(*forgotPass).Send(ctx.Response().Writer)
	return nil
}

func (c controller) UpdateUsername(ctx echo.Context) error {
	res := response.NewJSONResponse()
	aDto := new(oauthDto.UpdateUsernameRequest).GetFieldsUpdateUsername(ctx)
	if err := aDto.ValidateUsernameDto(); err != nil {
		res.SetError(response.ErrInvalidIntrospectRequest).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	_, err := c.BasicAuthClient(ctx)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	uuid, err := uuid.Parse(aDto.UUID)
	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	err = c.useCase.UpdateUsername(
		ctx.Request().Context(),
		aDto.ToModel(uuid), aDto.Username)

	if err != nil {
		res.SetError(err).SetMessage(err.Error()).Send(ctx.Response().Writer)
		return nil
	}

	res.APIStatusSuccess().Send(ctx.Response().Writer)
	return nil
}


// internal/oauth/delivery/http/oauth_http_router.go
package oauthHttpController

import (
	"github.com/labstack/echo/v4"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
)

type Router struct {
	controller oauthDomain.HttpController
}

func NewRouter(controller oauthDomain.HttpController) *Router {
	return &Router{
		controller: controller,
	}
}

func (r *Router) Register(e *echo.Group) {
	oauth := e.Group("/oauth")
	{
		oauth.POST("/tokens", r.controller.Tokens)
		oauth.POST("/introspect", r.controller.Introspect)
		e.POST("/register", r.controller.Register)
		e.POST("/change-password", r.controller.ChangePassword)
		e.POST("/forgot-password", r.controller.ForgotPassword)
		e.POST("/update-username", r.controller.UpdateUsername)
	}

}


// internal/oauth/delivery/kafka/consumer/consumer.go
package oauthKafkaConsumer

import (
	"context"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	kafkaConsumer "github.com/diki-haryadi/ztools/kafka/consumer"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"
)

type consumer struct {
	createEventReader *kafkaConsumer.Reader
}

func NewConsumer(r *kafkaConsumer.Reader) oauthDomain.KafkaConsumer {
	return &consumer{createEventReader: r}
}

func (c *consumer) RunConsumers(ctx context.Context) {
	go c.createEvent(ctx, 2)
}

func (c *consumer) createEvent(ctx context.Context, workersNum int) {
	r := c.createEventReader.Client
	defer func() {
		if err := r.Close(); err != nil {
			logger.Zap.Sugar().Errorf("error closing create article consumer")
		}
	}()

	logger.Zap.Sugar().Infof("Starting consumer group: %v", r.Config().GroupID)

	workerChan := make(chan bool)
	worker := wrapper.BuildChain(
		c.createEventWorker(workerChan),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)
	for i := 0; i <= workersNum; i++ {
		go worker.ToWorkerFunc(ctx, nil)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-workerChan:
			go worker.ToWorkerFunc(ctx, nil)
		}
	}
}


// internal/oauth/delivery/kafka/consumer/worker.go
package oauthKafkaConsumer

import (
	"context"
	"encoding/json"

	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/ztools/logger"
	"github.com/diki-haryadi/ztools/wrapper"
)

func (c *consumer) createEventWorker(
	workerChan chan bool,
) wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		defer func() {
			workerChan <- true
		}()
		for {
			msg, err := c.createEventReader.Client.FetchMessage(ctx)
			if err != nil {
				return nil, err
			}

			logger.Zap.Sugar().Infof(
				"Kafka Worker recieved message at topic/partition/offset %v/%v/%v: %s = %s\n",
				msg.Topic,
				msg.Partition,
				msg.Offset,
				string(msg.Key),
				string(msg.Value),
			)

			aDto := new(oauthDto.CreateArticleRequestDto)
			if err := json.Unmarshal(msg.Value, &aDto); err != nil {
				continue
			}

			if err := c.createEventReader.Client.CommitMessages(ctx, msg); err != nil {
				return nil, err
			}
		}
	}
}


// internal/oauth/delivery/kafka/producer/producer.go
package oauthKafkaProducer

import (
	"context"

	"github.com/segmentio/kafka-go"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	kafkaProducer "github.com/diki-haryadi/ztools/kafka/producer"
)

type producer struct {
	createWriter *kafkaProducer.Writer
}

func NewProducer(w *kafkaProducer.Writer) oauthDomain.KafkaProducer {
	return &producer{createWriter: w}
}

func (p *producer) PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error {
	return p.createWriter.Client.WriteMessages(ctx, messages...)
}


// internal/oauth/domain/model/access_token.go
package oauthDomain

import (
	"database/sql"
	"time"
)

type AccessToken struct {
	Common
	ClientID  sql.NullString `db:"client_id"`
	UserID    sql.NullString `db:"user_id"`
	Client    *Client
	User      *Users
	Token     string    `sql:"token"`
	ExpiresAt time.Time `sql:"expires_at"`
	Scope     string    `sql:"scope"`
}


// internal/oauth/domain/model/authorization_code.go
package oauthDomain

import (
	"database/sql"
	"time"
)

type AuthorizationCode struct {
	Common
	ClientID    sql.NullString `db:"client_id"`
	UserID      sql.NullString `db:"user_id"`
	Client      *Client
	User        *Users
	Code        string         `sql:"code"`
	RedirectURI sql.NullString `db:"redirect_uri"`
	ExpiresAt   time.Time      `sql:"expires_at"`
	Scope       string         `sql:"scope"`
}


// internal/oauth/domain/model/client.go
package oauthDomain

import "database/sql"

type Client struct {
	Common
	Key         string         `db:"key" json:"key"`
	Secret      string         `db:"secret" json:"secret"`
	RedirectURI sql.NullString `db:"redirect_uri" json:"redirect_uri"`
}


// internal/oauth/domain/model/common.go
package oauthDomain

import (
	"github.com/google/uuid"
	"time"
)

type Common struct {
	ID        uuid.UUID  `db:"id"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type Timestamp struct {
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type EmailTokenModel struct {
	Common
	Reference   string     `db:"reference"`
	EmailSent   bool       `db:"email_sent"`
	EmailSentAt *time.Time `db:"email_sent_at"`
	ExpiresAt   time.Time  `db:"expires_at"`
}


// internal/oauth/domain/model/refresh_token.go
package oauthDomain

import (
	"database/sql"
	"time"
)

type RefreshToken struct {
	Common
	ClientID  sql.NullString `db:"client_id"`
	UserID    sql.NullString `db:"user_id"`
	Client    *Client
	User      *Users
	Token     string    `sql:"token"`
	ExpiresAt time.Time `sql:"expires_at"`
	Scope     string    `sql:"scope"`
}


// internal/oauth/domain/model/role.go
package oauthDomain

import "github.com/google/uuid"

type Role struct {
	ID   uuid.UUID `db:"id" json:"id"`
	Name string    `db:"name" json:"name"`
	Timestamp
}


// internal/oauth/domain/model/scope.go
package oauthDomain

type Scope struct {
	Common
	Scope       string `db:"scope" json:"scope"`
	Description string `db:"description" json:"desc"`
	IsDefault   bool   `db:"is_default"`
}


// internal/oauth/domain/model/session.go
package oauthDomain

import "errors"

var (
	// StorageSessionName ...
	StorageSessionName = "go_oauth2_server_session"
	// UserSessionKey ...
	UserSessionKey = "go_oauth2_server_user"
	// ErrSessonNotStarted ...
	ErrSessonNotStarted = errors.New("Session not started")
)

type UserSession struct {
	ClientID     string
	Username     string
	AccessToken  string
	RefreshToken string
}


// internal/oauth/domain/model/token.go
package oauthDomain

import (
	"database/sql"
	"time"
)

type Token struct {
	Common
	ClientID  sql.NullString `db:"client_id"`
	UserID    sql.NullString `db:"user_id"`
	Client    *Client
	User      *Users
	Token     string    `sql:"token"`
	ExpiresAt time.Time `sql:"expires_at"`
	Scope     string    `sql:"scope"`
}


// internal/oauth/domain/model/user.go
package oauthDomain

import "database/sql"

type Users struct {
	Common
	RoleID   sql.NullString `db:"role_id" json:"role_id"`
	Role     *Role
	Username string         `db:"username" json:"username"`
	Password sql.NullString `db:"password" json:"password"`
}


// internal/oauth/domain/oauth_domain.go
package oauthDomain

import (
	"context"
	model "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	oauth2 "github.com/diki-haryadi/protobuf-ecomerce/oauth2_server_service/oauth2/v1"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
)

type Configurator interface {
	Configure(ctx context.Context) error
}

type UseCase interface {
	AuthClient(ctx context.Context, clientID, secret string) (*model.Client, error)
	CreateClient(ctx context.Context, clientID, secret, redirectURI string) (*model.Client, error)
	ClientExists(ctx context.Context, clientID string) bool
	ClientCredentialsGrant(ctx context.Context, scope string, refreshToken string, client *model.Client) (*oauthDto.AccessTokenResponse, error)
	AuthorizationCodeGrant(ctx context.Context, code, redirectURI string, client *model.Client) (*oauthDto.AccessTokenResponse, error)
	PasswordGrant(ctx context.Context, username, password string, scope string, client *model.Client) (*oauthDto.AccessTokenResponse, error)
	RefreshTokenGrant(ctx context.Context, token, scope string, client *model.Client) (*oauthDto.AccessTokenResponse, error)
	IntrospectToken(ctx context.Context, token, tokenTypeHint string, client *model.Client) (*oauthDto.IntrospectResponse, error)
	Login(ctx context.Context, client *model.Client, user *model.Users, scope string) (*model.AccessToken, *model.RefreshToken, error)
	GetRefreshTokenScope(ctx context.Context, refreshToken *model.RefreshToken, requestedScope string) (string, error)
	GrantAccessToken(ctx context.Context, client *model.Client, user *model.Users, expiresIn int, scope string) (*oauthDto.AccessTokenResponse, error)
	Register(ctx context.Context, dto *oauthDto.UserRequestDto) (*oauthDto.UserResponse, error)
	ChangePassword(ctx context.Context, dto *oauthDto.ChangePasswordRequest) (*oauthDto.ChangePasswordResponse, error)
	ForgotPassword(ctx context.Context, dto *oauthDto.ForgotPasswordRequest) (*oauthDto.ForgotPasswordResponse, error)
	UpdateUsername(ctx context.Context, user *model.Users, username string) error
}

type Repository interface {
	GrantAccessToken(ctx context.Context, client *model.Client, user *model.Users, expiresIn int, scope string) (*model.AccessToken, error)
	Authenticate(ctx context.Context, token string) (*model.AccessToken, error)
	GrantAuthorizationCode(ctx context.Context, client *model.Client, user *model.Users, expiresIn int, redirectURI, scope string) (*model.AuthorizationCode, error)
	GetValidAuthorizationCode(ctx context.Context, code, redirectURI string, client *model.Client) (*model.AuthorizationCode, error)
	CreateClientCommon(ctx context.Context, clientID, secret, redirectURI string) (*model.Client, error)
	FindClientByClientID(ctx context.Context, clientID string) (*model.Client, error)
	FetchAuthorizationCodeByCode(ctx context.Context, client *model.Client, code string) (*model.AuthorizationCode, error)
	DeleteAuthorizationCode(ctx context.Context, authorizationCodeID string) error
	FetchClientByClientID(ctx context.Context, clientID string) (*model.Client, error)
	FetchUserByUserID(ctx context.Context, userID string) (*model.Users, error)
	GetOrCreateRefreshToken(ctx context.Context, client *model.Client, user *model.Users, expiresIn int, scope string) (*model.RefreshToken, error)
	GetValidRefreshToken(ctx context.Context, token string, client *model.Client) (*model.RefreshToken, error)
	FindRoleByID(ctx context.Context, id string) (*model.Role, error)
	GetScope(ctx context.Context, requestedScope string) (string, error)
	GetDefaultScope(ctx context.Context) string
	ScopeExists(ctx context.Context, requestedScope string) bool
	FindUserByUsername(ctx context.Context, username string) (*model.Users, error)
	CreateUserCommon(ctx context.Context, roleID, username, password string) (*model.Users, error)
	SetPasswordCommon(ctx context.Context, user *model.Users, password string) error
	UpdateUsernameCommon(ctx context.Context, user *model.Users, username string) error
	UpdatePassword(ctx context.Context, uuid, password string) error
}

type GrpcController interface {
	PasswordGrant(ctx context.Context, req *oauth2.PasswordGrantRequest) (*oauth2.PasswordGrantResponse, error)
	AuthorizationCodeGrant(ctx context.Context, req *oauth2.AuthorizationCodeGrantRequest) (*oauth2.AuthorizationCodeGrantResponse, error)
	ClientCredentialsGrant(ctx context.Context, req *oauth2.ClientCredentialsGrantRequest) (*oauth2.ClientCredentialsGrantResponse, error)
	RefreshTokenGrant(ctx context.Context, req *oauth2.RefreshTokenGrantRequest) (*oauth2.RefreshTokenGrantResponse, error)
}

type HttpController interface {
	Tokens(c echo.Context) error
	Introspect(c echo.Context) error
	Register(c echo.Context) error
	ChangePassword(c echo.Context) error
	ForgotPassword(c echo.Context) error
	UpdateUsername(ctx echo.Context) error
}

type Job interface {
	StartJobs(ctx context.Context)
}

type KafkaProducer interface {
	PublishCreateEvent(ctx context.Context, messages ...kafka.Message) error
}

type KafkaConsumer interface {
	RunConsumers(ctx context.Context)
}


// internal/oauth/dto/access_token_dto.go
package oauthDto

import (
	"fmt"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
)

// AccessTokenResponse ...
type AccessTokenResponse struct {
	UserID       string `json:"user_id,omitempty"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// NewAccessTokenResponse ...
func NewAccessTokenResponse(accessToken *oauthModel.AccessToken, refreshToken *oauthModel.RefreshToken, lifetime int, theTokenType string) (*AccessTokenResponse, error) {
	response := &AccessTokenResponse{
		AccessToken: accessToken.Token,
		ExpiresIn:   lifetime,
		TokenType:   theTokenType,
		Scope:       accessToken.Scope,
	}
	if accessToken.UserID.Valid {
		response.UserID = accessToken.UserID.String
	}
	if refreshToken != nil {
		response.RefreshToken = refreshToken.Token
	}
	return response, nil
}

func NewOauthAccessToken(client *oauthModel.Client, user *oauthModel.Users, expiresIn int, scope string) (*oauthModel.AccessToken, error) {
	tokenID := uuid.New()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)),
		},
		ClientID:  fmt.Sprint(client.ID),
		Scope:     scope,
		TokenType: "access_token",
	}

	if user != nil {
		claims.UserID = fmt.Sprint(user.ID)
	}

	token, err := generateJWTToken(claims, config.BaseConfig.App.ConfigOauth.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	accessToken := &oauthModel.AccessToken{
		Common: oauthModel.Common{
			ID:        tokenID,
			CreatedAt: time.Now().UTC(),
		},
		ClientID:  pkg.StringOrNull(fmt.Sprint(client.ID)),
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		Scope:     scope,
	}

	if user != nil {
		accessToken.UserID = pkg.StringOrNull(fmt.Sprint(user.ID))
	}

	return accessToken, nil
}


// internal/oauth/dto/authorization_code_grant_dto.go
package oauthDto

import (
	"fmt"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"time"
)

type AuthorizationCodeGrantRequestDto struct {
	GrantType   string `json:"grant_type"`
	Code        string `json:"code"`
	RedirectUri string `json:"redirect_uri"`
	ClientID    string `json:"client_id"`
}

func (g *AuthorizationCodeGrantRequestDto) GetFields(ctx echo.Context) *AuthorizationCodeGrantRequestDto {
	return &AuthorizationCodeGrantRequestDto{
		GrantType:   ctx.FormValue("grant_type"),
		Code:        ctx.FormValue("code"),
		RedirectUri: ctx.FormValue("redirect_uri"),
		ClientID:    ctx.FormValue("client_id"),
	}
}

func (g *AuthorizationCodeGrantRequestDto) GetFieldsValue(code string, redirectUri string, clientID string) *AuthorizationCodeGrantRequestDto {
	return &AuthorizationCodeGrantRequestDto{
		GrantType:   "-",
		Code:        code,
		RedirectUri: redirectUri,
		ClientID:    clientID,
	}
}

func (g *AuthorizationCodeGrantRequestDto) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *AuthorizationCodeGrantRequestDto) ValidateAuthorizationCodeDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.GrantType,
			validator.Required,
		),
		validator.Field(
			&caDto.Code,
			validator.Required,
		),
		validator.Field(
			&caDto.RedirectUri,
			validator.Required,
		),
	)
}

func NewOauthAuthorizationCode(client *oauthModel.Client, user *oauthModel.Users, expiresIn int, redirectURI, scope string) *oauthModel.AuthorizationCode {
	return &oauthModel.AuthorizationCode{
		Common: oauthModel.Common{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		},
		ClientID:    pkg.StringOrNull(fmt.Sprint(client.ID)),
		UserID:      pkg.StringOrNull(fmt.Sprint(user.ID)),
		Code:        uuid.New().String(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		RedirectURI: pkg.StringOrNull(redirectURI),
		Scope:       scope,
	}
}


// internal/oauth/dto/change_password.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ChangePasswordRequest struct {
	UUID        string `json:"uuid"`
	Password    string `json:"password"`
	NewPassword string `json:"new_password"`
}

func (g *ChangePasswordRequest) GetFieldsChangePassword(ctx echo.Context) *ChangePasswordRequest {
	return &ChangePasswordRequest{
		UUID:        ctx.FormValue("uuid"),
		Password:    ctx.FormValue("password"),
		NewPassword: ctx.FormValue("new_password"),
	}
}

func (g *ChangePasswordRequest) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *ChangePasswordRequest) ValidateChangePasswordDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
		validator.Field(
			&caDto.NewPassword,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type ChangePasswordResponse struct {
	Status bool `json:"status"`
}


// internal/oauth/dto/client_credentials_grant_dto.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ClientCredentialsGrantRequestDto struct {
	GrantType string `json:"grant_type"`
	Scope     string `json:"scope"`
}

func (g *ClientCredentialsGrantRequestDto) GetFields(ctx echo.Context) *ClientCredentialsGrantRequestDto {
	return &ClientCredentialsGrantRequestDto{
		GrantType: ctx.FormValue("grant_type"),
		Scope:     ctx.FormValue("scope"),
	}
}

func (g *ClientCredentialsGrantRequestDto) GetFieldsValue(scope string) *ClientCredentialsGrantRequestDto {
	return &ClientCredentialsGrantRequestDto{
		GrantType: "-",
		Scope:     scope,
	}
}

func (g *ClientCredentialsGrantRequestDto) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *ClientCredentialsGrantRequestDto) ValidateClientCredentialsDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.GrantType,
			validator.Required,
		),
		validator.Field(
			&caDto.Scope,
			validator.Required,
		),
	)
}


// internal/oauth/dto/forgot_password.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ForgotPasswordRequest struct {
	UUID     string `json:"uuid"`
	Password string `json:"password"`
}

func (g *ForgotPasswordRequest) GetFieldsForgotPassword(ctx echo.Context) *ForgotPasswordRequest {
	return &ForgotPasswordRequest{
		UUID:     ctx.FormValue("uuid"),
		Password: ctx.FormValue("password"),
	}
}

func (g *ForgotPasswordRequest) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *ForgotPasswordRequest) ValidateForgotPasswordDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type ForgotPasswordResponse struct {
	Status bool `json:"status"`
}


// internal/oauth/dto/introspect_dto.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type IntrospectRequestDto struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"token_type_hint"`
}

func (g *IntrospectRequestDto) GetFields(ctx echo.Context) *IntrospectRequestDto {
	return &IntrospectRequestDto{
		Token:         ctx.FormValue("token"),
		TokenTypeHint: ctx.FormValue("token_type_hint"),
	}
}

func (g *IntrospectRequestDto) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *IntrospectRequestDto) ValidateIntrospectDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.Token,
			validator.Required,
		),
		validator.Field(
			&caDto.TokenTypeHint,
			validator.Required,
		),
	)
}

// IntrospectResponse ...
type IntrospectResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	ExpiresAt int    `json:"exp,omitempty"`
	IssuedAt  int    `json:"iat,omitempty"`
	Sub       string `json:"sub,omitempty"`
	JTI       string `json:"jti,omitempty"`
}


// internal/oauth/dto/jwt_token.go
package oauthDto

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id,omitempty"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	TokenType string `json:"token_type"`
}

func generateJWTToken(claims TokenClaims, secretKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateToken(tokenString, secretKey string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}


// internal/oauth/dto/oauth_dto.go
package oauthDto

type TokenRequestDto struct {
	GrantType string `json:"grant_type"`
}


// internal/oauth/dto/password_grant_dto.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PasswordGrantRequestDto struct {
	GrantType string `json:"grant_type"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Scope     string `json:"scope"`
}

func (g *PasswordGrantRequestDto) GetFields(ctx echo.Context) *PasswordGrantRequestDto {
	return &PasswordGrantRequestDto{
		GrantType: ctx.FormValue("grant_type"),
		Username:  ctx.FormValue("username"),
		Password:  ctx.FormValue("password"),
		Scope:     ctx.FormValue("scope"),
	}
}

func (g *PasswordGrantRequestDto) GetFieldsValue(username string, password string, scope string) *PasswordGrantRequestDto {
	return &PasswordGrantRequestDto{
		GrantType: "-",
		Username:  username,
		Password:  password,
		Scope:     scope,
	}
}

func (g *PasswordGrantRequestDto) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *PasswordGrantRequestDto) ValidatePasswordDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.GrantType,
			validator.Required,
		),
		validator.Field(
			&caDto.Username,
			validator.Required,
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
		),
		validator.Field(
			&caDto.Scope,
			validator.Required,
		),
	)
}


// internal/oauth/dto/refresh_token_dto.go
package oauthDto

import (
	"fmt"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"time"
)

type RefreshTokenRequestDto struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (g *RefreshTokenRequestDto) GetFields(ctx echo.Context) *RefreshTokenRequestDto {
	return &RefreshTokenRequestDto{
		GrantType:    ctx.FormValue("grant_type"),
		RefreshToken: ctx.FormValue("refresh_token"),
		Scope:        ctx.FormValue("scope"),
	}
}

func (g *RefreshTokenRequestDto) GetFieldsValue(refreshToken string, scope string) *RefreshTokenRequestDto {
	return &RefreshTokenRequestDto{
		GrantType:    "-",
		RefreshToken: refreshToken,
		Scope:        scope,
	}
}

func (g *RefreshTokenRequestDto) ToModel(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *RefreshTokenRequestDto) ValidateRefreshTokenDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.GrantType,
			validator.Required,
		),
		validator.Field(
			&caDto.RefreshToken,
			validator.Required,
		),
		validator.Field(
			&caDto.Scope,
			validator.Required,
		),
	)
}

func NewOauthRefreshToken(client *oauthModel.Client, user *oauthModel.Users, expiresIn int, scope string) (*oauthModel.RefreshToken, error) {
	tokenID := uuid.New()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)),
		},
		ClientID:  fmt.Sprint(client.ID),
		Scope:     scope,
		TokenType: "refresh_token",
	}

	if user != nil {
		claims.UserID = fmt.Sprint(user.ID)
	}

	token, err := generateJWTToken(claims, config.BaseConfig.App.ConfigOauth.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	refreshToken := &oauthModel.RefreshToken{
		Common: oauthModel.Common{
			ID:        tokenID,
			CreatedAt: time.Now().UTC(),
		},
		ClientID:  pkg.StringOrNull(fmt.Sprint(client.ID)),
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		Scope:     scope,
	}

	if user != nil {
		refreshToken.UserID = pkg.StringOrNull(fmt.Sprint(user.ID))
	}

	return refreshToken, nil
}


// internal/oauth/dto/register_dto.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type UserRequestDto struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   string `json:"role_id"`
}

func (g *UserRequestDto) GetFieldsUser(ctx echo.Context) *UserRequestDto {
	return &UserRequestDto{
		Username: ctx.FormValue("username"),
		Password: ctx.FormValue("password"),
		RoleID:   ctx.FormValue("role_id"),
	}
}

func (g *UserRequestDto) ToModelUser(clientID uuid.UUID) *oauthModel.Client {
	return &oauthModel.Client{
		Common: oauthModel.Common{
			ID: clientID,
		},
	}
}

func (caDto *UserRequestDto) ValidateUserDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.Username,
			validator.Required,
			validator.Length(5, 50),
		),
		validator.Field(
			&caDto.Password,
			validator.Required,
			validator.Length(6, 30),
		),
		validator.Field(
			&caDto.RoleID,
			validator.Required,
		),
	)
}

// UserResponse ...
type UserResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}


// internal/oauth/dto/update_username.go
package oauthDto

import (
	oauthModel "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type UpdateUsernameRequest struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

func (g *UpdateUsernameRequest) GetFieldsUpdateUsername(ctx echo.Context) *UpdateUsernameRequest {
	return &UpdateUsernameRequest{
		UUID:     ctx.FormValue("uuid"),
		Username: ctx.FormValue("username"),
	}
}

func (g *UpdateUsernameRequest) ToModel(userID uuid.UUID) *oauthModel.Users {
	return &oauthModel.Users{
		Common: oauthModel.Common{
			ID: userID,
		},
	}
}

func (caDto *UpdateUsernameRequest) ValidateUsernameDto() error {
	return validator.ValidateStruct(caDto,
		validator.Field(
			&caDto.UUID,
			validator.Required,
		),
		validator.Field(
			&caDto.Username,
			validator.Required,
			validator.Length(6, 30),
		),
	)
}

type UsernameResponse struct {
	Status bool `json:"status"`
}


// internal/oauth/exception/oauth_exception.go
package oauthException

import (
	errorList "github.com/diki-haryadi/go-micro-template/pkg/constant/error/error_list"
	customErrors "github.com/diki-haryadi/go-micro-template/pkg/error/custom_error"
	errorUtils "github.com/diki-haryadi/go-micro-template/pkg/error/error_utils"
)

func AuthorizationCodeGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func AuthorizationCodeGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func PasswordGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func PasswordGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func GrantClientCredentialGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func GrantClientCredentialGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func RefreshTokenGrantValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func RefreshTokenGrantBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}

func IntrospectValidationExc(err error) error {
	ve, ie := errorUtils.ValidationErrorHandler(err)
	if ie != nil {
		return ie
	}

	validationError := errorList.InternalErrorList.ValidationError
	return customErrors.NewValidationError(validationError.Msg, validationError.Code, ve)
}

func IntrospectBindingExc() error {
	oauthBindingError := errorList.InternalErrorList.OauthExceptions.BindingError
	return customErrors.NewBadRequestError(oauthBindingError.Msg, oauthBindingError.Code, nil)
}


// internal/oauth/job/job.go
package oauthJob

import (
	"context"

	"go.uber.org/zap"

	"github.com/robfig/cron/v3"

	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	"github.com/diki-haryadi/ztools/wrapper"
	wrapperErrorhandler "github.com/diki-haryadi/ztools/wrapper/handlers/error_handler"
	wrapperRecoveryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/recovery_handler"
	wrapperSentryHandler "github.com/diki-haryadi/ztools/wrapper/handlers/sentry_handler"

	cronJob "github.com/diki-haryadi/ztools/cron"
)

type job struct {
	cron   *cron.Cron
	logger *zap.Logger
}

func NewJob(logger *zap.Logger) oauthDomain.Job {
	newCron := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cronJob.NewLogger()),
	))
	return &job{cron: newCron, logger: logger}
}

func (j *job) StartJobs(ctx context.Context) {
	j.logArticleJob(ctx)
	go j.cron.Start()
}

func (j *job) logArticleJob(ctx context.Context) {
	worker := wrapper.BuildChain(j.logArticleWorker(),
		wrapperSentryHandler.SentryHandler,
		wrapperRecoveryHandler.RecoveryHandler,
		wrapperErrorhandler.ErrorHandler,
	)

	entryId, _ := j.cron.AddFunc("*/1 * * * *",
		worker.ToWorkerFunc(ctx, nil),
	)

	j.logger.Sugar().Infof("Article Job Started: %v", entryId)
}


// internal/oauth/job/worker.go
package oauthJob

import (
	"context"

	"github.com/diki-haryadi/ztools/wrapper"
)

func (j *job) logArticleWorker() wrapper.HandlerFunc {
	return func(ctx context.Context, args ...interface{}) (interface{}, error) {
		j.logger.Info("article log job")
		return nil, nil
	}
}


// internal/oauth/repository/access_token.go
package oauthRepository

import (
	"context"
	"fmt"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"time"
)

func (rp *repository) GrantAccessToken(ctx context.Context, client *oauthDomain.Client, user *oauthDomain.Users, expiresIn int, scope string) (*oauthDomain.AccessToken, error) {
	// Begin a transaction
	tx, err := rp.postgres.SqlxDB.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	rawQuery := " WHERE client_id = $1"
	args := []interface{}{fmt.Sprint(client.ID)}

	// Add the user_id condition if necessary
	if user != nil && fmt.Sprint(user) != "" {
		rawQuery += " AND user_id = $2"
		args = append(args, user.ID) // Add user.ID to the arguments list
	} else {
		rawQuery += " AND user_id IS NULL"
	}

	// Add the expiration condition
	rawQuery += " AND expires_at <= NOW()"

	// Complete the DELETE statement
	query := "DELETE FROM access_tokens" + rawQuery

	// Execute the query using parameterized arguments
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		// If an error occurs, rollback the transaction
		_ = tx.Rollback()
		return nil, err
	}

	// Create a new access token
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	accessToken, err := oauthDto.NewOauthAccessToken(client, user, expiresIn, scope)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	var sqlQueryAT string
	var insertArgs []interface{}

	if user != nil {
		sqlQueryAT = `
        INSERT INTO access_tokens (client_id, user_id, token, expires_at, scope)
        VALUES ($1, $2, $3, $4, $5)`
		insertArgs = append(insertArgs, fmt.Sprint(client.ID), user.ID)
	} else {
		sqlQueryAT = `
        INSERT INTO access_tokens (client_id, token, expires_at, scope)
        VALUES ($1, $2, $3, $4)`
		insertArgs = append(insertArgs, fmt.Sprint(client.ID))
	}

	insertArgs = append(insertArgs, accessToken.Token, expiresAt, scope)
	_, err = tx.ExecContext(ctx, sqlQueryAT, insertArgs...)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	accessToken.Client = client
	accessToken.User = user

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	return accessToken, nil
}


// internal/oauth/repository/authenticate.go
package oauthRepository

import (
	"context"
	"database/sql"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"time"
)

// Authenticate checks the access token is valid
func (rp *repository) Authenticate(ctx context.Context, token string) (*oauthDomain.AccessToken, error) {
	// 1. Fetch the access token from the database using a SELECT query
	sqlQuery := "SELECT id, token, client_id, user_id, expires_at FROM access_tokens WHERE token = $1"
	row := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, token)

	// 2. Scan the results into an AccessToken object
	accessToken := new(oauthDomain.AccessToken)
	err := row.Scan(&accessToken.ID, &accessToken.Token, &accessToken.ClientID, &accessToken.UserID, &accessToken.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrAccessTokenNotFound
		}
		return nil, err
	}

	// 3. Check if the access token has expired
	if time.Now().UTC().After(accessToken.ExpiresAt) {
		return nil, response.ErrAccessTokenExpired
	}

	// 4. Extend the refresh token expiration
	// Use an UPDATE query to extend the expiration time for the refresh token
	refreshTokenQuery := `
        UPDATE refresh_tokens
        SET expires_at = $1
        WHERE client_id = $2 AND (user_id = $3 OR user_id IS NULL)
    `

	increasedExpiresAt := time.Now().UTC().Add(time.Duration(config.BaseConfig.App.ConfigOauth.Oauth.RefreshTokenLifetime) * time.Second)
	var userID interface{}
	if accessToken.UserID.Valid {
		userID = accessToken.UserID.String
	} else {
		userID = nil
	}

	// Execute the query to update the refresh token expiration
	_, err = rp.postgres.SqlxDB.ExecContext(ctx, refreshTokenQuery, increasedExpiresAt, accessToken.ClientID.String, userID)
	if err != nil {
		return nil, err
	}

	// Return the fetched access token
	return accessToken, nil
}

func (rp *repository) ClearUserTokens(ctx context.Context, userSession *oauthDomain.UserSession) {
	// 1. Check if the refresh token exists in the database
	tx, _ := rp.postgres.SqlxDB.BeginTx(context.Background(), nil)
	var refreshToken oauthDomain.RefreshToken
	sqlQuery := "SELECT * FROM refresh_tokens WHERE token = $1"
	row := tx.QueryRowContext(ctx, sqlQuery, userSession.RefreshToken)

	// 2. If refresh token is found, delete associated records with client_id and user_id
	err := row.Scan(&refreshToken.ID, &refreshToken.Token, &refreshToken.ClientID, &refreshToken.UserID)
	if err == nil { // Username found
		// Perform delete operation for refresh tokens
		deleteQuery := "DELETE FROM refresh_tokens WHERE client_id = $1 AND user_id = $2"
		_, err = tx.ExecContext(ctx, deleteQuery, refreshToken.ClientID.String, refreshToken.UserID.String)
		if err != nil {
			tx.Rollback()
			return
		}
	}

	// 3. Check if the access token exists in the database
	var accessToken oauthDomain.AccessToken
	sqlQuery = "SELECT * FROM access_tokens WHERE token = $1"
	row = tx.QueryRowContext(ctx, sqlQuery, userSession.AccessToken)

	// 4. If access token is found, delete associated records with client_id and user_id
	err = row.Scan(&accessToken.ID, &accessToken.Token, &accessToken.ClientID, &accessToken.UserID)
	if err == nil { // Username found
		// Perform delete operation for access tokens
		deleteQuery := "DELETE FROM access_tokens WHERE client_id = $1 AND user_id = $2"
		_, err = tx.ExecContext(ctx, deleteQuery, accessToken.ClientID.String, accessToken.UserID.String)
		if err != nil {
			tx.Rollback()
			return
		}
	}
}


// internal/oauth/repository/authorization_code.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"time"
)

// GrantAuthorizationCode grants a new authorization code using raw SQL
func (rp *repository) GrantAuthorizationCode(ctx context.Context, client *oauthDomain.Client, user *oauthDomain.Users, expiresIn int, redirectURI, scope string) (*oauthDomain.AuthorizationCode, error) {
	// Generate a new authorization code
	authorizationCode := oauthDto.NewOauthAuthorizationCode(client, user, expiresIn, redirectURI, scope)

	// Prepare the SQL INSERT query to insert the new authorization code into the database
	sqlQuery := `
        INSERT INTO authorization_codes (client_id, user_id, code, redirect_uri, expires_at, scope)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, client_id, user_id, code, redirect_uri, expires_at, scope
    `

	// Execute the query and retrieve the generated ID and other fields
	row := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, client.ID, user.ID, authorizationCode.Code, authorizationCode.RedirectURI.String, authorizationCode.ExpiresAt, authorizationCode.Scope)

	// Map the result into the authorizationCode object
	err := row.Scan(&authorizationCode.ID, &authorizationCode.ClientID, &authorizationCode.UserID, &authorizationCode.Code, &authorizationCode.RedirectURI, &authorizationCode.ExpiresAt, &authorizationCode.Scope)
	if err != nil {
		return nil, err
	}

	// Set the associated client and user (these are already set from the input)
	authorizationCode.Client = client
	authorizationCode.User = user

	return authorizationCode, nil
}

// getValidAuthorizationCode returns a valid non-expired authorization code using raw SQL
func (rp *repository) GetValidAuthorizationCode(ctx context.Context, code, redirectURI string, client *oauthDomain.Client) (*oauthDomain.AuthorizationCode, error) {
	// Fetch the authorization code from the database using raw SQL query
	sqlQuery := `
        SELECT id, client_id, user_id, code, redirect_uri, expires_at, scope
        FROM authorization_codes
        WHERE client_id = $1 AND code = $2
    `

	row := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, client.ID, code)

	// Scan the result into an authorizationCode object
	authorizationCode := new(oauthDomain.AuthorizationCode)
	err := row.Scan(&authorizationCode.ID, &authorizationCode.ClientID, &authorizationCode.UserID, &authorizationCode.Code, &authorizationCode.RedirectURI, &authorizationCode.ExpiresAt, &authorizationCode.Scope)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrAuthorizationCodeNotFound
		}
		return nil, err
	}

	// Check if the redirect URI matches
	if redirectURI != authorizationCode.RedirectURI.String {
		return nil, response.ErrInvalidRedirectURI
	}

	// Check if the authorization code has expired
	if time.Now().After(authorizationCode.ExpiresAt) {
		return nil, response.ErrAuthorizationCodeExpired
	}

	return authorizationCode, nil
}


// internal/oauth/repository/client_repo.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (rp *repository) CreateClientCommon(ctx context.Context, clientID, secret, redirectURI string) (*oauthDomain.Client, error) {
	// 1. Check if client already exists
	var existingClient oauthDomain.Client
	sqlCheck := `SELECT id FROM clients WHERE client_id = $1`
	err := rp.postgres.SqlxDB.GetContext(ctx, &existingClient, sqlCheck, clientID)
	if err == nil {
		return nil, response.ErrClientIDTaken // Client ID is already taken
	}
	if err != sql.ErrNoRows {
		return nil, err // Other errors
	}

	// 2. Hash the secret (password)
	secretHash, err := pkg.HashPassword(secret)
	if err != nil {
		return nil, err
	}

	// 3. Insert the new client into the database
	sqlInsert := `
        INSERT INTO clients (client_id, secret, redirect_uri, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id, client_id, secret, redirect_uri, created_at
    `

	client := &oauthDomain.Client{
		Common: oauthDomain.Common{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		},
		Key:         strings.ToLower(clientID),
		Secret:      string(secretHash),
		RedirectURI: pkg.StringOrNull(redirectURI),
	}

	// Execute the insert query and scan the results into the client struct
	err = rp.postgres.SqlxDB.QueryRowContext(ctx, sqlInsert, client.Key, client.Secret, client.RedirectURI, client.CreatedAt).Scan(
		&client.ID, &client.Key, &client.Secret, &client.RedirectURI, &client.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (rp *repository) FindClientByClientID(ctx context.Context, clientID string) (*oauthDomain.Client, error) {
	client := oauthDomain.Client{}
	query := "SELECT * FROM clients WHERE key = $1"
	err := rp.postgres.SqlxDB.GetContext(ctx, &client, query, strings.ToLower(clientID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrClientNotFound
		}
		return nil, err
	}

	return &client, err
}


// internal/oauth/repository/grant_type_authorization_code.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
)

// FetchAuthorizationCodeByCode retrieves the authorization code from the database using raw SQL
func (rp *repository) FetchAuthorizationCodeByCode(ctx context.Context, client *oauthDomain.Client, code string) (*oauthDomain.AuthorizationCode, error) {
	sqlQuery := `
        SELECT ac.id, ac.client_id, ac.user_id, ac.code, ac.redirect_uri, ac.expires_at, ac.scope, r.name AS role_name
		FROM authorization_codes ac
		JOIN users u ON ac.user_id::UUID = u.id
		JOIN roles r ON u.role_id::UUID = r.id
        WHERE client_id = $1 AND code = $2`

	var authorizationCode oauthDomain.AuthorizationCode
	var user oauthDomain.Users
	var role oauthDomain.Role
	var cl oauthDomain.Client

	row := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, client.ID, code)

	// Scan the result into the authorizationCode struct
	err := row.Scan(
		&authorizationCode.ID,
		&authorizationCode.ClientID,
		&authorizationCode.UserID,
		&authorizationCode.Code,
		&authorizationCode.RedirectURI,
		&authorizationCode.ExpiresAt,
		&authorizationCode.Scope,
		&role.Name,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrAuthorizationCodeNotFound
		}
		return nil, err
	}

	cl.ID = client.ID
	u, _ := uuid.Parse(authorizationCode.UserID.String)
	user.ID = u
	user.Role = &role

	authorizationCode.User = &user
	authorizationCode.Client = &cl

	return &authorizationCode, nil
}

// DeleteAuthorizationCode deletes the authorization code from the database after use
func (rp *repository) DeleteAuthorizationCode(ctx context.Context, authorizationCodeID string) error {
	sqlDelete := `
        DELETE FROM authorization_codes WHERE id = $1
    `
	_, err := rp.postgres.SqlxDB.ExecContext(ctx, sqlDelete, authorizationCodeID)
	return err
}


// internal/oauth/repository/introspect.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// FetchClientByClientID retrieves the client by client_id using raw SQL
func (rp *repository) FetchClientByClientID(ctx context.Context, clientID string) (*oauthDomain.Client, error) {
	sqlClientQuery := "SELECT key FROM clients WHERE id = $1"
	client := new(oauthDomain.Client)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlClientQuery, clientID).Scan(&client.Key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrClientNotFound
		}
		return nil, err
	}
	return client, nil
}

// FetchUserByUserID retrieves the user by user_id using raw SQL
func (rp *repository) FetchUserByUserID(ctx context.Context, userID string) (*oauthDomain.Users, error) {
	sqlUserQuery := "SELECT id, username, password FROM users WHERE id = $1"
	user := new(oauthDomain.Users)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlUserQuery, userID).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, response.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}


// internal/oauth/repository/oauth_repo.go
package oauthRepository

import (
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
	"github.com/diki-haryadi/ztools/postgres"
)

type repository struct {
	postgres *postgres.Postgres
}

func NewRepository(conn *postgres.Postgres) oauthDomain.Repository {
	return &repository{postgres: conn}
}


// internal/oauth/repository/refresh_token.go
package oauthRepository

import (
	"context"
	"database/sql"
	"fmt"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"time"
)

// GetOrCreateRefreshToken retrieves an existing refresh token, if expired,
// the token gets deleted and a new refresh token is created using raw SQL
func (rp *repository) GetOrCreateRefreshToken(ctx context.Context, client *oauthDomain.Client, user *oauthDomain.Users, expiresIn int, scope string) (*oauthDomain.RefreshToken, error) {
	// Try to fetch an existing refresh token first using raw SQL
	var refreshToken oauthDomain.RefreshToken
	sqlQuery := "SELECT id, client_id, user_id, token, expires_at, scope FROM refresh_tokens WHERE client_id = $1"

	if user != nil && fmt.Sprint(user.ID) != "" {
		sqlQuery += " AND user_id = $2"
	} else {
		sqlQuery += " AND user_id IS NULL"
	}

	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, client.ID, user.ID).Scan(
		&refreshToken.ID,
		&refreshToken.ClientID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.Scope,
	)

	var expired bool
	if err == nil {
		// Check if the token is expired
		expired = time.Now().UTC().After(refreshToken.ExpiresAt)
	}

	// If the refresh token has expired or does not exist, delete it
	if expired || err == sql.ErrNoRows {
		if err == nil { // If token exists, delete it
			sqlDelete := "DELETE FROM refresh_tokens WHERE id = $1"
			_, err = rp.postgres.SqlxDB.ExecContext(ctx, sqlDelete, refreshToken.ID)
			if err != nil {
				return nil, err
			}
		}

		// Create a new refresh token if it expired or was not found
		refreshTokenNew, err := oauthDto.NewOauthRefreshToken(client, user, expiresIn, scope)
		if err != nil {
			return nil, err
		}

		sqlInsert := `
            INSERT INTO refresh_tokens (client_id, user_id, token, expires_at, scope)
            VALUES ($1, $2, $3, $4, $5)
            RETURNING id, client_id, user_id, token, expires_at, scope`
		fmt.Println(refreshToken)
		err = rp.postgres.SqlxDB.QueryRowContext(ctx, sqlInsert, refreshToken.ClientID, refreshToken.UserID, refreshToken.Token, refreshToken.ExpiresAt, refreshToken.Scope).
			Scan(&refreshToken.ID, &refreshToken.ClientID, &refreshToken.UserID, &refreshToken.Token, &refreshToken.ExpiresAt, &refreshToken.Scope)

		if err != nil {
			return nil, err
		}
		return refreshTokenNew, nil
	}

	return &refreshToken, nil
}

// GetValidRefreshToken returns a valid non expired refresh token using raw SQL
func (rp *repository) GetValidRefreshToken(ctx context.Context, token string, client *oauthDomain.Client) (*oauthDomain.RefreshToken, error) {
	// Fetch the refresh token from the database using raw SQL
	var refreshToken oauthDomain.RefreshToken
	sqlQuery := "SELECT id, client_id, user_id, token, expires_at, scope FROM refresh_tokens WHERE client_id = $1 AND token = $2"

	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, client.ID, token).Scan(
		&refreshToken.ID,
		&refreshToken.ClientID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.Scope,
	)

	// Not found
	if err == sql.ErrNoRows {
		return nil, response.ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	// Check if the refresh token hasn't expired
	if time.Now().UTC().After(refreshToken.ExpiresAt) {
		return nil, response.ErrRefreshTokenExpired
	}

	return &refreshToken, nil
}


// internal/oauth/repository/role.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// FindRoleByID retrieves a role by its ID using raw SQL
func (rp *repository) FindRoleByID(ctx context.Context, id string) (*oauthDomain.Role, error) {
	sqlQuery := "SELECT id, name FROM roles WHERE id = $1"

	role := new(oauthDomain.Role)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, id).Scan(&role.ID, &role.Name)

	if err == sql.ErrNoRows {
		return nil, response.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}

	return role, nil
}


// internal/oauth/repository/scope.go
package oauthRepository

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"sort"
	"strconv"
	"strings"
)

// GetScope takes a requested scope and, if it's empty, returns the default
// scope, if not empty, it validates the requested scope
func (rp *repository) GetScope(ctx context.Context, requestedScope string) (string, error) {
	// Return the default scope if the requested scope is empty
	if requestedScope == "" {
		return rp.GetDefaultScope(ctx), nil
	}

	// If the requested scope exists in the database, return it
	if rp.ScopeExists(ctx, requestedScope) {
		return requestedScope, nil
	}

	// Otherwise return error
	return "", response.ErrInvalidScope
}

// GetDefaultScope retrieves the default scope from the database using raw SQL
func (rp *repository) GetDefaultScope(ctx context.Context) string {
	// Fetch default scopes from the database using raw SQL
	sqlQuery := "SELECT scope FROM scopes WHERE is_default = $1"
	rows, err := rp.postgres.SqlxDB.QueryContext(ctx, sqlQuery, true)
	if err != nil {
		// Handle error (e.g., database connection issues)
		return ""
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			// Handle error (e.g., scanning issues)
			return ""
		}
		scopes = append(scopes, scope)
	}

	// Sort the scopes alphabetically
	sort.Strings(scopes)

	// Return space-delimited scope string
	return strings.Join(scopes, " ")
}

// ScopeExists checks if a scope exists using raw SQL
func (rp *repository) ScopeExists(ctx context.Context, requestedScope string) bool {
	scopes := strings.Split(requestedScope, ",")

	query := "SELECT COUNT(*) FROM scopes WHERE scope IN ("

	placeholders := make([]string, len(scopes))
	for i := range scopes {
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}
	query += strings.Join(placeholders, ", ") + ")"

	var count int
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, query, scopes).Scan(&count)
	if err != nil {
		return false
	}

	return count == len(scopes)
}


// internal/oauth/repository/user.go
package oauthRepository

import (
	"context"
	"database/sql"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (rp *repository) FindUserByUsername(ctx context.Context, username string) (*oauthDomain.Users, error) {
	sqlQuery := "SELECT id, username, password, role_id, created_at, updated_at FROM users WHERE LOWER(username) = $1"

	user := new(oauthDomain.Users)
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlQuery, strings.ToLower(username)).Scan(
		&user.ID, &user.Username, &user.Password, &user.RoleID, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, response.ErrUserNotFound
	}
	if err != nil {
		return nil, err // Handle any other error
	}

	return user, nil
}

func (rp *repository) CreateUserCommon(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	user := &oauthDomain.Users{
		Common: oauthDomain.Common{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
		},
		RoleID:   pkg.StringOrNull(roleID),
		Username: strings.ToLower(username),
		Password: pkg.StringOrNull(""),
	}

	// If the password is being set, hash it
	if password != "" {
		if len(password) < response.MinPasswordLength {
			return nil, response.ErrPasswordTooShort
		}

		passwordHash, err := pkg.HashPassword(password)
		if err != nil {
			return nil, err
		}
		user.Password = pkg.StringOrNull(string(passwordHash))
	}

	// Check if the username is already taken using raw SQL
	sqlCheckUsername := "SELECT COUNT(*) FROM users WHERE LOWER(username) = $1"
	var count int
	err := rp.postgres.SqlxDB.QueryRowContext(ctx, sqlCheckUsername, user.Username).Scan(&count)
	if err != nil {
		return nil, err
	}

	if count > 0 {
		return nil, response.ErrUsernameTaken
	}

	// Insert the new user into the database
	sqlInsert := `
        INSERT INTO users (id, created_at, role_id, username, password)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, role_id, username, password
    `
	err = rp.postgres.SqlxDB.QueryRowContext(ctx, sqlInsert, user.ID, user.CreatedAt, user.RoleID, user.Username, user.Password).
		Scan(&user.ID, &user.CreatedAt, &user.RoleID, &user.Username, &user.Password)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// SetPasswordCommon updates the user's password using raw SQL
func (rp *repository) SetPasswordCommon(ctx context.Context, user *oauthDomain.Users, password string) error {
	if len(password) < response.MinPasswordLength {
		return response.ErrPasswordTooShort
	}

	// Create a bcrypt hash for the password
	passwordHash, err := pkg.HashPassword(password)
	if err != nil {
		return err
	}

	// Prepare the SQL query to update the password and the updated_at field
	sqlQuery := `
        UPDATE users
        SET password = $1, updated_at = $2
        WHERE id = $3
    `

	// Execute the query to update the user's password
	_, err = rp.postgres.SqlxDB.ExecContext(ctx, sqlQuery, string(passwordHash), time.Now().UTC(), user.ID)
	if err != nil {
		return err
	}

	return nil
}

// UpdateUsernameCommon updates the user's username using raw SQL
func (rp *repository) UpdateUsernameCommon(ctx context.Context, user *oauthDomain.Users, username string) error {
	if username == "" {
		return response.ErrCannotSetEmptyUsername
	}

	// Prepare the SQL query to update the username field
	sqlQuery := `
        UPDATE users
        SET username = $1
        WHERE id = $2`

	// Execute the query to update the username
	_, err := rp.postgres.SqlxDB.ExecContext(ctx, sqlQuery, strings.ToLower(username), user.ID)
	if err != nil {
		return err
	}

	return nil
}

func (rp *repository) UpdatePassword(ctx context.Context, uuid, password string) error {
	if password == "" {
		return response.ErrUserPasswordNotSet
	}

	// Prepare the SQL query to update the username field
	sqlQuery := `
        UPDATE users
        SET password = $1
        WHERE id = $2`

	// Execute the query to update the username
	_, err := rp.postgres.SqlxDB.Exec(sqlQuery, password, uuid)
	if err != nil {
		return err
	}

	return nil
}


// internal/oauth/tests/fixtures/oauth_integration_fixture.go
package oauthFixture

import (
	"context"
	"math"
	"net"
	"time"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	sampleExtServiceUseCase "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/usecase"
	oauthGrpc "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/grpc"
	oauthHttp "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/http"
	oauthKafkaProducer "github.com/diki-haryadi/go-micro-template/internal/oauth/delivery/kafka/producer"
	oauthRepo "github.com/diki-haryadi/go-micro-template/internal/oauth/repository"
	oauthUseCase "github.com/diki-haryadi/go-micro-template/internal/oauth/usecase"
	externalBridge "github.com/diki-haryadi/ztools/external_bridge"
	iContainer "github.com/diki-haryadi/ztools/infra_container"
	"github.com/diki-haryadi/ztools/logger"
)

const BUFSIZE = 1024 * 1024

type IntegrationTestFixture struct {
	TearDown          func()
	Ctx               context.Context
	Cancel            context.CancelFunc
	InfraContainer    *iContainer.IContainer
	ArticleGrpcClient articleV1.ArticleServiceClient
}

func NewIntegrationTestFixture() (*IntegrationTestFixture, error) {
	deadline := time.Now().Add(time.Duration(math.MaxInt64))
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	container := iContainer.IContainer{}
	ic, infraDown, err := container.IContext(ctx).
		ICDown().ICPostgres().ICGrpc().ICEcho().
		ICKafka().NewIC()
	if err != nil {
		cancel()
		return nil, err
	}

	extBridge, extBridgeDown, err := externalBridge.NewExternalBridge(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	seServiceUseCase := sampleExtServiceUseCase.NewSampleExtServiceUseCase(extBridge.SampleExtGrpcService)
	kafkaProducer := oauthKafkaProducer.NewProducer(ic.KafkaWriter)
	repository := oauthRepo.NewRepository(ic.Postgres)
	useCase := oauthUseCase.NewUseCase(repository, seServiceUseCase, kafkaProducer)

	// http
	ic.EchoHttpServer.SetupDefaultMiddlewares()
	httpRouterGp := ic.EchoHttpServer.GetEchoInstance().Group(ic.EchoHttpServer.GetBasePath())
	httpController := oauthHttp.NewController(useCase)
	oauthHttp.NewRouter(httpController).Register(httpRouterGp)

	// grpc
	grpcController := oauthGrpc.NewController(useCase)
	articleV1.RegisterArticleServiceServer(ic.GrpcServer.GetCurrentGrpcServer(), grpcController)

	lis := bufconn.Listen(BUFSIZE)
	go func() {
		if err := ic.GrpcServer.GetCurrentGrpcServer().Serve(lis); err != nil {
			logger.Zap.Sugar().Fatalf("Server exited with error: %v", err)
		}
	}()

	grpcClientConn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	articleGrpcClient := articleV1.NewArticleServiceClient(grpcClientConn)

	return &IntegrationTestFixture{
		TearDown: func() {
			cancel()
			infraDown()
			_ = grpcClientConn.Close()
			extBridgeDown()
		},
		InfraContainer:    ic,
		Ctx:               ctx,
		Cancel:            cancel,
		ArticleGrpcClient: articleGrpcClient,
	}, nil
}


// internal/oauth/tests/integrations/create_oauth_test.go
package artcileIntegrationTest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	articleV1 "github.com/diki-haryadi/protobuf-template/go-micro-template/article/v1"
	"github.com/labstack/echo/v4"

	articleDto "github.com/diki-haryadi/go-micro-template/internal/article/dto"
	articleFixture "github.com/diki-haryadi/go-micro-template/internal/article/tests/fixtures"
	grpcError "github.com/diki-haryadi/ztools/error/grpc"
	httpError "github.com/diki-haryadi/ztools/error/http"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type testSuite struct {
	suite.Suite
	fixture *articleFixture.IntegrationTestFixture
}

func (suite *testSuite) SetupSuite() {
	fixture, err := articleFixture.NewIntegrationTestFixture()
	if err != nil {
		assert.Error(suite.T(), err)
	}

	suite.fixture = fixture
}

func (suite *testSuite) TearDownSuite() {
	suite.fixture.TearDown()
}

func (suite *testSuite) TestSuccessfulCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro Developer",
	}

	response, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)
	if err != nil {
		assert.Error(suite.T(), err)
	}

	assert.NotNil(suite.T(), response.Id)
	assert.Equal(suite.T(), "John", response.Name)
	assert.Equal(suite.T(), "Pro Developer", response.Desc)
}

func (suite *testSuite) TestNameValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "Jo",
		Desc: "Pro Developer",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "name")
}

func (suite *testSuite) TestDescValidationErrCreateGrpcArticle() {
	ctx := context.Background()

	createArticleRequest := &articleV1.CreateArticleRequest{
		Name: "John",
		Desc: "Pro",
	}
	_, err := suite.fixture.ArticleGrpcClient.CreateArticle(ctx, createArticleRequest)

	assert.NotNil(suite.T(), err)

	grpcErr := grpcError.ParseExternalGrpcErr(err)
	assert.NotNil(suite.T(), grpcErr)
	assert.Equal(suite.T(), codes.InvalidArgument, grpcErr.GetStatus())
	assert.Contains(suite.T(), grpcErr.GetDetails(), "desc")
}

func (suite *testSuite) TestSuccessCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusOK, response.Code)

	caDto := new(articleDto.CreateArticleRequestDto)
	if assert.NoError(suite.T(), json.Unmarshal(response.Body.Bytes(), caDto)) {
		assert.Equal(suite.T(), "John Snow", caDto.Name)
		assert.Equal(suite.T(), "King of the north", caDto.Description)
	}

}

func (suite *testSuite) TestNameValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"Jo","desc":"King of the north"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()
	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "name")
	}

}

func (suite *testSuite) TestDescValidationErrCreateHttpArticle() {
	articleJSON := `{"name":"John Snow","desc":"King"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/article", strings.NewReader(articleJSON))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	response := httptest.NewRecorder()

	suite.fixture.InfraContainer.EchoHttpServer.SetupDefaultMiddlewares()
	suite.fixture.InfraContainer.EchoHttpServer.GetEchoInstance().ServeHTTP(response, request)

	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)

	httpErr := httpError.ParseExternalHttpErr(response.Result().Body)
	if assert.NotNil(suite.T(), httpErr) {
		assert.Equal(suite.T(), http.StatusBadRequest, httpErr.GetStatus())
		assert.Contains(suite.T(), httpErr.GetDetails(), "desc")
	}
}

func TestRunSuite(t *testing.T) {
	tSuite := new(testSuite)
	suite.Run(t, tSuite)
}


// internal/oauth/usecase/change_password.go
package oauthUseCase

import (
	"context"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) ChangePassword(ctx context.Context, dto *oauthDto.ChangePasswordRequest) (*oauthDto.ChangePasswordResponse, error) {
	if len(dto.Password) < response.MinPasswordLength {
		return &oauthDto.ChangePasswordResponse{}, response.ErrPasswordTooShort
	}

	user, err := uc.repository.FetchUserByUserID(ctx, dto.UUID)
	if err != nil {
		return &oauthDto.ChangePasswordResponse{}, nil
	}

	err = pkg.VerifyPassword(user.Password.String, dto.Password)
	if err != nil {
		return &oauthDto.ChangePasswordResponse{}, response.ErrInvalidPassword
	}

	passwordHash, err := pkg.HashPassword(dto.NewPassword)
	if err != nil {
		return &oauthDto.ChangePasswordResponse{}, err
	}

	err = uc.repository.UpdatePassword(ctx, dto.UUID, string(passwordHash))
	if err != nil {
		return &oauthDto.ChangePasswordResponse{}, nil
	}

	return &oauthDto.ChangePasswordResponse{
		Status: true,
	}, nil
}


// internal/oauth/usecase/client_usecase.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) AuthClient(ctx context.Context, clientID, secret string) (*oauthDomain.Client, error) {
	client, err := uc.repository.FindClientByClientID(ctx, clientID)
	if err != nil {
		return nil, response.ErrClientNotFound
	}

	if pkg.VerifyPassword(client.Secret, secret) != nil {
		return nil, response.ErrInvalidClientSecret
	}
	return client, nil
}

func (uc *useCase) CreateClient(ctx context.Context, clientID, secret, redirectURI string) (*oauthDomain.Client, error) {
	client, err := uc.repository.CreateClientCommon(ctx, clientID, secret, redirectURI)
	if err != nil {
		return nil, err
	}
	return client, err
}

func (uc *useCase) ClientExists(ctx context.Context, clientID string) bool {
	_, err := uc.repository.FindClientByClientID(ctx, clientID)
	return err == nil
}


// internal/oauth/usecase/forgot_password.go
package oauthUseCase

import (
	"context"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) ForgotPassword(ctx context.Context, dto *oauthDto.ForgotPasswordRequest) (*oauthDto.ForgotPasswordResponse, error) {
	if len(dto.Password) < response.MinPasswordLength {
		return &oauthDto.ForgotPasswordResponse{}, response.ErrPasswordTooShort
	}

	user, err := uc.repository.FetchUserByUserID(ctx, dto.UUID)
	if err != nil {
		return &oauthDto.ForgotPasswordResponse{}, nil
	}

	err = pkg.VerifyPassword(user.Password.String, dto.Password)
	if err == nil {
		return &oauthDto.ForgotPasswordResponse{}, response.ErrInvalidPasswordCannotSame
	}

	passwordHash, err := pkg.HashPassword(dto.Password)
	if err != nil {
		return &oauthDto.ForgotPasswordResponse{}, err
	}

	err = uc.repository.UpdatePassword(ctx, dto.UUID, string(passwordHash))
	if err != nil {
		return &oauthDto.ForgotPasswordResponse{}, nil
	}

	return &oauthDto.ForgotPasswordResponse{
		Status: true,
	}, nil
}


// internal/oauth/usecase/grant_access_token.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
)

func (uc *useCase) GrantAccessToken(ctx context.Context, client *oauthDomain.Client, user *oauthDomain.Users, expiresIn int, scope string) (*oauthDto.AccessTokenResponse, error) {
	accessToken, err := uc.repository.GrantAccessToken(ctx, client, user, expiresIn, scope)
	if err != nil {
		return nil, err
	}

	accessTokenResponse, err := oauthDto.NewAccessTokenResponse(
		accessToken,
		nil,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime, // expires
		pkg.Bearer,
	)
	return accessTokenResponse, nil
}


// internal/oauth/usecase/grant_type_authorization_code.go
package oauthUseCase

import (
	"context"
	"fmt"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"time"
)

func (uc *useCase) AuthorizationCodeGrant(ctx context.Context, code, redirectURI string, client *oauthDomain.Client) (*oauthDto.AccessTokenResponse, error) {
	// 1. Fetch the authorization code from the database
	authorizationCode, err := uc.repository.FetchAuthorizationCodeByCode(ctx, client, code)
	if err != nil {
		return nil, err
	}

	// 2. Check if redirect URI matches
	if redirectURI != authorizationCode.RedirectURI.String {
		return nil, response.ErrInvalidRedirectURI
	}

	// 3. Check if the authorization code has expired
	if time.Now().After(authorizationCode.ExpiresAt) {
		return nil, response.ErrAuthorizationCodeExpired
	}

	// 4. Log in the user
	accessToken, refreshToken, err := uc.Login(ctx, authorizationCode.Client, authorizationCode.User, authorizationCode.Scope)
	if err != nil {
		return nil, err
	}

	// 5. Delete the authorization code from the database
	err = uc.repository.DeleteAuthorizationCode(ctx, fmt.Sprint(authorizationCode.ID))
	if err != nil {
		return nil, err
	}

	// 6. Create the access token response
	accessTokenResponse, err := oauthDto.NewAccessTokenResponse(
		accessToken,
		refreshToken,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime,
		pkg.Bearer,
	)
	if err != nil {
		return nil, err
	}

	return accessTokenResponse, nil
}


// internal/oauth/usecase/grant_type_client_credentials.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) ClientCredentialsGrant(ctx context.Context, scope string, refreshToken string, client *oauthDomain.Client) (*oauthDto.AccessTokenResponse, error) {
	// Fetch the refresh token
	theRefreshToken, err := uc.repository.GetValidRefreshToken(ctx, refreshToken, client)
	if err != nil {
		return nil, err
	}

	// Get the scope
	scope, err = uc.getRefreshTokenScope(ctx, theRefreshToken, scope)
	if err != nil {
		return nil, err
	}

	// Log in the user
	accessToken, newRefreshToken, err := uc.Login(
		ctx,
		theRefreshToken.Client,
		theRefreshToken.User,
		scope,
	)
	if err != nil {
		return nil, err
	}

	// Create response
	accessTokenResponse, err := oauthDto.NewAccessTokenResponse(
		accessToken,
		newRefreshToken, // refresh token
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime,
		pkg.Bearer,
	)
	if err != nil {
		return nil, err
	}

	return accessTokenResponse, nil
}

func (uc *useCase) getRefreshTokenScope(ctx context.Context, refreshToken *oauthDomain.RefreshToken, requestedScope string) (string, error) {
	var (
		scope = refreshToken.Scope // default to the scope originally granted by the resource owner
		err   error
	)

	// If the scope is specified in the request, get the scope string
	if requestedScope != "" {
		scope, err = uc.repository.GetScope(ctx, requestedScope)
		if err != nil {
			return "", err
		}
	}

	// Requested scope CANNOT include any scope not originally granted
	if !pkg.SpaceDelimitedStringNotGreater(scope, refreshToken.Scope) {
		return "", response.ErrRequestedScopeCannotBeGreater
	}

	return scope, nil
}


// internal/oauth/usecase/grant_type_password.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) PasswordGrant(ctx context.Context, username, password string, scope string, client *oauthDomain.Client) (*oauthDto.AccessTokenResponse, error) {
	// Get the scope string
	scope, err := uc.GetScope(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Authenticate the user
	user, err := uc.AuthUser(ctx, username, password)
	if err != nil {
		// For security reasons, return a general error message
		return nil, response.ErrInvalidUsernameOrPassword
	}

	// Log in the user
	accessToken, refreshToken, err := uc.Login(ctx, client, user, scope)
	if err != nil {
		return nil, err
	}

	// Create response
	accessTokenResponse, err := oauthDto.NewAccessTokenResponse(
		accessToken,
		refreshToken,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime,
		pkg.Bearer,
	)
	if err != nil {
		return nil, err
	}

	return accessTokenResponse, nil
}


// internal/oauth/usecase/grant_type_refresh_token.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
)

func (uc *useCase) RefreshTokenGrant(ctx context.Context, token, scope string, client *oauthDomain.Client) (*oauthDto.AccessTokenResponse, error) {
	// Fetch the refresh token
	theRefreshToken, err := uc.repository.GetValidRefreshToken(ctx, token, client)
	if err != nil {
		return nil, err
	}

	// Get the scope
	scopeR, err := uc.GetRefreshTokenScope(ctx, theRefreshToken, scope)
	if err != nil {
		return nil, err
	}

	// Log in the user
	accessToken, refreshToken, err := uc.Login(ctx,
		theRefreshToken.Client,
		theRefreshToken.User,
		scopeR,
	)
	if err != nil {
		return nil, err
	}

	// Create response
	accessTokenResponse, err := oauthDto.NewAccessTokenResponse(
		accessToken,
		refreshToken,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime, // expires
		pkg.Bearer,
	)
	if err != nil {
		return nil, err
	}

	return accessTokenResponse, nil
}


// internal/oauth/usecase/introspect.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	oauthDto "github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/constant"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"time"
)

func (uc *useCase) IntrospectToken(ctx context.Context, token, tokenTypeHint string, client *oauthDomain.Client) (*oauthDto.IntrospectResponse, error) {
	if tokenTypeHint == "" {
		tokenTypeHint = constant.AccessTokenHint
	}

	claims, err := oauthDto.ValidateToken(token, config.BaseConfig.App.ConfigOauth.JWTSecret)
	if err != nil {
		return &oauthDto.IntrospectResponse{Active: false}, nil
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return &oauthDto.IntrospectResponse{Active: false}, nil
	}

	switch tokenTypeHint {
	case constant.AccessTokenHint:
		if claims.TokenType != "access_token" {
			return &oauthDto.IntrospectResponse{Active: false}, nil
		}
		accessToken, err := uc.repository.Authenticate(ctx, token)
		if err != nil {
			return &oauthDto.IntrospectResponse{Active: false}, nil
		}
		return uc.NewIntrospectResponseFromAccessToken(ctx, accessToken, claims)

	case constant.RefreshTokenHint:
		if claims.TokenType != "refresh_token" {
			return &oauthDto.IntrospectResponse{Active: false}, nil
		}
		refreshToken, err := uc.repository.GetValidRefreshToken(ctx, token, client)
		if err != nil {
			return &oauthDto.IntrospectResponse{Active: false}, nil
		}
		return uc.NewIntrospectResponseFromRefreshToken(ctx, refreshToken, claims)

	default:
		return nil, response.ErrTokenHintInvalid
	}
}

func (uc *useCase) NewIntrospectResponseFromAccessToken(ctx context.Context, accessToken *oauthDomain.AccessToken, claims *oauthDto.TokenClaims) (*oauthDto.IntrospectResponse, error) {
	introspectResponse := &oauthDto.IntrospectResponse{
		Active:    true,
		Scope:     claims.Scope,
		TokenType: pkg.Bearer,
		ExpiresAt: int(claims.ExpiresAt.Unix()),
		IssuedAt:  int(claims.IssuedAt.Unix()),
		JTI:       claims.ID,
	}

	if claims.ClientID != "" {
		client, err := uc.repository.FetchClientByClientID(ctx, claims.ClientID)
		if err != nil {
			return nil, err
		}
		introspectResponse.ClientID = client.Key
	}

	if claims.UserID != "" {
		user, err := uc.repository.FetchUserByUserID(ctx, claims.UserID)
		if err != nil {
			return nil, err
		}
		introspectResponse.Username = user.Username
		introspectResponse.Sub = claims.UserID
	}

	return introspectResponse, nil
}

func (uc *useCase) NewIntrospectResponseFromRefreshToken(ctx context.Context, refreshToken *oauthDomain.RefreshToken, claims *oauthDto.TokenClaims) (*oauthDto.IntrospectResponse, error) {
	introspectResponse := &oauthDto.IntrospectResponse{
		Active:    true,
		Scope:     claims.Scope,
		TokenType: "refresh_token",
		ExpiresAt: int(claims.ExpiresAt.Unix()),
		IssuedAt:  int(claims.IssuedAt.Unix()),
		JTI:       claims.ID,
	}

	if claims.ClientID != "" {
		client, err := uc.repository.FetchClientByClientID(ctx, claims.ClientID)
		if err != nil {
			return nil, err
		}
		introspectResponse.ClientID = client.Key
	}

	if claims.UserID != "" {
		user, err := uc.repository.FetchUserByUserID(ctx, claims.UserID)
		if err != nil {
			return nil, err
		}
		introspectResponse.Username = user.Username
		introspectResponse.Sub = claims.UserID
	}

	return introspectResponse, nil
}


// internal/oauth/usecase/login.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// Login creates an access token and refresh token for a user (logs him/her in)
func (uc *useCase) Login(ctx context.Context, client *oauthDomain.Client, user *oauthDomain.Users, scope string) (*oauthDomain.AccessToken, *oauthDomain.RefreshToken, error) {
	if !uc.IsRoleAllowed(user.Role.Name) {
		return nil, nil, response.ErrInvalidUsernameOrPassword
	}

	// Create a new access token
	accessToken, err := uc.repository.GrantAccessToken(
		ctx,
		client,
		user,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime, // expires in
		scope,
	)
	if err != nil {
		return nil, nil, err
	}

	// Create or retrieve a refresh token
	refreshToken, err := uc.repository.GetOrCreateRefreshToken(ctx,
		client,
		user,
		config.BaseConfig.App.ConfigOauth.Oauth.AccessTokenLifetime, // expires in
		scope,
	)
	if err != nil {
		return nil, nil, err
	}

	return accessToken, refreshToken, nil
}


// internal/oauth/usecase/oauth_usecase.go
package oauthUseCase

import (
	sampleExtServiceDomain "github.com/diki-haryadi/go-micro-template/external/sample_ext_service/domain"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain"
)

type useCase struct {
	repository              oauthDomain.Repository
	kafkaProducer           oauthDomain.KafkaProducer
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase
	allowedRoles            []string
}

func NewUseCase(
	repository oauthDomain.Repository,
	sampleExtServiceUseCase sampleExtServiceDomain.SampleExtServiceUseCase,
	kafkaProducer oauthDomain.KafkaProducer,
) oauthDomain.UseCase {
	return &useCase{
		repository:              repository,
		kafkaProducer:           kafkaProducer,
		sampleExtServiceUseCase: sampleExtServiceUseCase,
		allowedRoles:            []string{Superuser, User},
	}
}


// internal/oauth/usecase/refresh_token.go
package oauthUseCase

import (
	"context"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// GetRefreshTokenScope returns scope for a new refresh token
func (uc *useCase) GetRefreshTokenScope(ctx context.Context, refreshToken *oauthDomain.RefreshToken, requestedScope string) (string, error) {
	var (
		scope = refreshToken.Scope // default to the scope originally granted by the resource owner
		err   error
	)

	// If the scope is specified in the request, get the scope string
	if requestedScope != "" {
		scope, err = uc.repository.GetScope(ctx, requestedScope)
		if err != nil {
			return "", err
		}
	}

	// Requested scope CANNOT include any scope not originally granted
	if !pkg.SpaceDelimitedStringNotGreater(scope, refreshToken.Scope) {
		return "", response.ErrRequestedScopeCannotBeGreater
	}

	return scope, nil
}


// internal/oauth/usecase/register.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/internal/oauth/dto"
)

func (uc *useCase) Register(ctx context.Context, dto *oauthDto.UserRequestDto) (*oauthDto.UserResponse, error) {
	user, err := uc.repository.CreateUserCommon(ctx, dto.RoleID, dto.Username, dto.Password)
	if err != nil {
		return &oauthDto.UserResponse{}, nil
	}
	return &oauthDto.UserResponse{
		Username: user.Username,
		Role:     user.Role.Name,
	}, nil
}


// internal/oauth/usecase/roles.go
package oauthUseCase

import (
	"errors"
	"strings"
)

const (
	// Superuser ...
	Superuser = "superuser"
	// User ...
	User = "user"
)

var roleWeight = map[string]int{
	Superuser: 100,
	User:      1,
}

// IsGreaterThan returns true if role1 is greater than role2
func (uc *useCase) IsGreaterThan(role1, role2 string) (bool, error) {
	// Get weight of the first role
	weight1, ok := roleWeight[role1]
	if !ok {
		return false, errors.New("Role weight not found")
	}

	// Get weight of the second role
	weight2, ok := roleWeight[role2]
	if !ok {
		return false, errors.New("Role weight not found")
	}

	return weight1 > weight2, nil
}

// RestrictToRoles restricts this service to only specified roles
func (uc *useCase) RestrictToRoles(allowedRoles ...string) {
	uc.allowedRoles = allowedRoles
}

// IsRoleAllowed returns true if the role is allowed to use this service
func (uc *useCase) IsRoleAllowed(role string) bool {
	for _, allowedRole := range uc.allowedRoles {
		if strings.ToLower(role) == allowedRole {
			return true
		}
	}
	return false
}


// internal/oauth/usecase/scope_usecase.go
package oauthUseCase

import (
	"context"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

func (uc *useCase) GetScope(ctx context.Context, requestScope string) (string, error) {
	if requestScope == "" {
		scope := uc.repository.GetDefaultScope(ctx)
		return scope, nil
	}

	if scope := uc.repository.ScopeExists(ctx, requestScope); scope {
		return requestScope, nil
	}
	return "", response.ErrInvalidScope
}


// internal/oauth/usecase/session.go
package oauthUseCase

import (
	"encoding/gob"
	"errors"
	"github.com/diki-haryadi/go-micro-template/config"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg/constant"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
	"github.com/gorilla/sessions"
	"net/http"
)

type Service struct {
	sessionStore   sessions.Store
	sessionOptions *sessions.Options
	session        *sessions.Session
	r              *http.Request
	w              http.ResponseWriter
}

func init() {
	// Register a new datatype for storage in sessions
	gob.Register(new(oauthDomain.UserSession))
}

// NewService returns a new Service instance
func NewService(cnf *config.Config, sessionStore sessions.Store) *Service {
	return &Service{
		// Session cookie storage
		sessionStore: sessionStore,
		// Session options
		sessionOptions: &sessions.Options{
			Path:     cnf.App.ConfigOauth.Session.Path,
			MaxAge:   cnf.App.ConfigOauth.Session.MaxAge,
			HttpOnly: cnf.App.ConfigOauth.Session.HTTPOnly,
		},
	}
}

// SetSessionService sets the request and responseWriter on the session service
func (s *Service) SetSessionService(r *http.Request, w http.ResponseWriter) {
	s.r = r
	s.w = w
}

// StartSession starts a new session. This method must be called before other
// public methods of this struct as it sets the internal session object
func (s *Service) StartSession() error {
	session, err := s.sessionStore.Get(s.r, constant.StorageSessionName)
	if err != nil {
		return err
	}
	s.session = session
	return nil
}

// GetUserSession returns the user session
func (s *Service) GetUserSession() (*oauthDomain.UserSession, error) {
	// Make sure StartSession has been called
	if s.session == nil {
		return nil, response.ErrSessonNotStarted
	}

	// Retrieve our user session struct and type-assert it
	userSession, ok := s.session.Values[constant.UserSessionKey].(*oauthDomain.UserSession)
	if !ok {
		return nil, errors.New("User session type assertion error")
	}

	return userSession, nil
}

// SetUserSession saves the user session
func (s *Service) SetUserSession(userSession *oauthDomain.UserSession) error {
	// Make sure StartSession has been called
	if s.session == nil {
		return response.ErrSessonNotStarted
	}

	// Set a new user session
	s.session.Values[constant.UserSessionKey] = userSession
	return s.session.Save(s.r, s.w)
}

// ClearUserSession deletes the user session
func (s *Service) ClearUserSession() error {
	// Make sure StartSession has been called
	if s.session == nil {
		return response.ErrSessonNotStarted
	}

	// Delete the user session
	delete(s.session.Values, constant.UserSessionKey)
	return s.session.Save(s.r, s.w)
}

// SetFlashMessage sets a flash message,
// useful for displaying an error after 302 redirection
func (s *Service) SetFlashMessage(msg string) error {
	// Make sure StartSession has been called
	if s.session == nil {
		return response.ErrSessonNotStarted
	}

	// Add the flash message
	s.session.AddFlash(msg)
	return s.session.Save(s.r, s.w)
}

// GetFlashMessage returns the first flash message
func (s *Service) GetFlashMessage() (interface{}, error) {
	// Make sure StartSession has been called
	if s.session == nil {
		return nil, response.ErrSessonNotStarted
	}

	// Get the last flash message from the stack
	if flashes := s.session.Flashes(); len(flashes) > 0 {
		// We need to save the session, otherwise the flash message won't be removed
		s.session.Save(s.r, s.w)
		return flashes[0], nil
	}

	// No flash messages in the stack
	return nil, nil
}

// Close stops any running services
func (s *useCase) Close() {}


// internal/oauth/usecase/users_usecase.go
package oauthUseCase

import (
	"context"
	oauthDomain "github.com/diki-haryadi/go-micro-template/internal/oauth/domain/model"
	"github.com/diki-haryadi/go-micro-template/pkg"
	"github.com/diki-haryadi/go-micro-template/pkg/response"
)

// UserExists returns true if user exists
func (uc *useCase) UserExists(ctx context.Context, username string) bool {
	_, err := uc.repository.FindUserByUsername(ctx, username)
	return err == nil
}

// CreateUser saves a new user to database
func (uc *useCase) CreateUser(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	return uc.repository.CreateUserCommon(ctx, roleID, username, password)
}

// CreateUserTx saves a new user to database using injected db object
func (uc *useCase) CreateUserTx(ctx context.Context, roleID, username, password string) (*oauthDomain.Users, error) {
	return uc.repository.CreateUserCommon(ctx, roleID, username, password)
}

// SetPassword sets a user password
func (uc *useCase) SetPassword(ctx context.Context, user *oauthDomain.Users, password string) error {
	return uc.repository.SetPasswordCommon(ctx, user, password)
}

// SetPasswordTx sets a user password in a transaction
func (uc *useCase) SetPasswordTx(ctx context.Context, user *oauthDomain.Users, password string) error {
	return uc.repository.SetPasswordCommon(ctx, user, password)
}

// AuthUser authenticates user
func (uc *useCase) AuthUser(ctx context.Context, username, password string) (*oauthDomain.Users, error) {
	// Fetch the user
	user, err := uc.repository.FindUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// Check that the password is set
	if !user.Password.Valid {
		return nil, err
	}

	// Verify the password
	if pkg.VerifyPassword(user.Password.String, password) != nil {
		return nil, err
	}

	role, err := uc.repository.FindRoleByID(ctx, user.RoleID.String)
	if err != nil {
		return nil, err
	}
	user.Role = role
	return user, nil
}

// UpdateUsername ...
func (uc *useCase) UpdateUsername(ctx context.Context, user *oauthDomain.Users, username string) error {
	if username == "" {
		return response.ErrCannotSetEmptyUsername
	}

	return uc.repository.UpdateUsernameCommon(ctx, user, username)
}

// UpdateUsernameTx ...
func (uc *useCase) UpdateUsernameTx(ctx context.Context, user *oauthDomain.Users, username string) error {
	return uc.repository.UpdateUsernameCommon(ctx, user, username)
}


// main.go
package main

import "github.com/diki-haryadi/go-micro-template/cmd"

func main() {
	cmd.Execute()
}


// pkg/constant/constant-temp.go
package constant

type (
	PassAlgorithm string

	DB string

	IsDeleted int

	ResourcesType int

	KeyID string

	ContextKey string

	CacheKey string

	Gender int
)

var (
	Bcrypt PassAlgorithm = "bcrypt"
	MD5    PassAlgorithm = "md5"
	Argon  PassAlgorithm = "argon"
	SHA    PassAlgorithm = "sha"

	MySQL      DB = "mysql"
	PostgreSQL DB = "postgres"

	False IsDeleted = 0
	True  IsDeleted = 1

	Menu ResourcesType = 1
	API  ResourcesType = 2

	Default KeyID = "default"

	Claim ContextKey = "claim"

	UserAuth      CacheKey = "auth::user-uid:%s"
	RoleMenu      CacheKey = "resources::role-uid:%s:type:%d"
	JWKPrivateKey CacheKey = "jwk::private-key:%s"
	MenuResource  CacheKey = "resources::type:%d"

	Female Gender = 0
	Male   Gender = 1
)

func (pa PassAlgorithm) String() string {
	return string(pa)
}

func (db DB) String() string {
	return string(db)
}

func (i IsDeleted) Int() int {
	return int(i)
}

func (k KeyID) String() string {
	return string(k)
}

func (rt ResourcesType) Int() int {
	return int(rt)
}

func (rt ResourcesType) String() string {
	switch rt.Int() {
	case Menu.Int():
		return "menu"
	case API.Int():
		return "api"
	default:
		return " "
	}
}

func ResourceTypeAtoi(s string) ResourcesType {
	switch s {
	case "menu":
		return Menu
	case "api":
		return API
	default:
		return 0
	}
}

func (ct ContextKey) String() string {
	return string(ct)
}

func (g Gender) Int() int {
	return int(g)
}

func (g Gender) String() string {
	switch g.Int() {
	case Female.Int():
		return "female"
	case Male.Int():
		return "male"
	default:
		return ""
	}
}


// pkg/constant/constant.go
package constant

// App
const AppName = "Go-Microservice-Template"

const (
	AppEnvProd = "prod"
	AppEnvDev  = "dev"
	AppEnvTest = "test"
)

// Http + Grpc
const (
	HttpHost      = "localhost"
	HttpPort      = 4000
	EchoGzipLevel = 5

	GrpcHost = "localhost"
	GrpcPort = 3000
)

// Postgres
const (
	PgMaxConn         = 1
	PgMaxIdleConn     = 1
	PgMaxLifeTimeConn = 1
	PgSslMode         = "disable"
)

const (
	AccessTokenHint  = "access_token"
	RefreshTokenHint = "refresh_token"
)

const (
	StorageSessionName = "go_oauth2_server_session"
	UserSessionKey     = "go_oauth2_server_user"
)


// pkg/constant/error/error_list/error_list.go
package errorList

var InternalErrorList *internalErrorList

type internalErrorList struct {
	ValidationError     ErrorList
	InternalServerError ErrorList
	NotFoundError       ErrorList
	OauthExceptions     OauthErrorList
}

type ErrorList struct {
	Msg  string
	Code int
}

type OauthErrorList struct {
	BindingError ErrorList
}

func init() {
	InternalErrorList = &internalErrorList{
		// 1000 - 1999 : BoilerPlate Err
		// 2000 - 2999 : Custom Err Per Service
		// .
		// .
		// .
		// 8000 - 8999 : Third-party
		// 9000 - 9999 : FATAL

		InternalServerError: ErrorList{
			Msg:  "internal server error",
			Code: 1000,
		},

		ValidationError: ErrorList{
			Msg:  "request validation failed",
			Code: 1001,
		},

		NotFoundError: ErrorList{
			Msg:  "not found",
			Code: 1002,
		},

		OauthExceptions: OauthErrorList{
			BindingError: ErrorList{
				Msg:  "binding failed",
				Code: 3002,
			},
		},
	}
}


// pkg/constant/error/error_title.go
package errorConstant

const (
	ErrBadRequestTitle          = "Bad Request"
	ErrConflictTitle            = "Conflict Error"
	ErrNotFoundTitle            = "Not Found"
	ErrUnauthorizedTitle        = "Unauthorized"
	ErrForbiddenTitle           = "Forbidden"
	ErrRequestTimeoutTitle      = "Request Timeout"
	ErrInternalServerErrorTitle = "Internal Server Error"
	ErrDomainTitle              = "Domain Model Error"
	ErrApplicationTitle         = "Application Service Error"
	ErrApiTitle                 = "Api Error"
	ErrValidationFailedTitle    = "Validation Failed"
)


// pkg/constant/httputil.go
package constant

import (
	"net/http"
)

const (
	StatusCtxKey                = 0
	StatusSuccess               = http.StatusOK
	StatusErrorForm             = http.StatusBadRequest
	StatusErrorUnknown          = http.StatusBadGateway
	StatusInternalError         = http.StatusInternalServerError
	StatusUnauthorized          = http.StatusUnauthorized
	StatusCreated               = http.StatusCreated
	StatusAccepted              = http.StatusAccepted
	StatusNoContent             = http.StatusNoContent
	StatusForbidden             = http.StatusForbidden
	StatusInvalidAuthentication = http.StatusProxyAuthRequired
	StatusNotFound              = http.StatusNotFound
)

var statusMap = map[int][]string{
	StatusSuccess:               {"STATUS_OK", "Success"},
	StatusErrorForm:             {"STATUS_BAD_REQUEST", "Invalid data request"},
	StatusErrorUnknown:          {"STATUS_BAD_GATEWAY", "Oops something went wrong"},
	StatusInternalError:         {"INTERNAL_SERVER_ERROR", "Oops something went wrong"},
	StatusUnauthorized:          {"STATUS_UNAUTHORIZED", "Not authorized to access the service"},
	StatusCreated:               {"STATUS_CREATED", "Resource has been created"},
	StatusAccepted:              {"STATUS_ACCEPTED", "Resource has been accepted"},
	StatusNoContent:             {"STATUS_NO_CONTENT", "Resource has been delete"},
	StatusForbidden:             {"STATUS_FORBIDDEN", "Forbidden access the resource "},
	StatusInvalidAuthentication: {"STATUS_INVALID_AUTHENTICATION", "The resource owner or authorization server denied the request"},
	StatusNotFound:              {"STATUS_NOT_FOUND", "Not Found"},
}

func StatusCode(code int) string {
	return statusMap[code][0]
}

func StatusText(code int) string {
	return statusMap[code][1]
}


// pkg/constant/logger/logger.go
package loggerConstant

const (
	GRPC   = "GRPC"
	HTTP   = "HTTP"
	WORKER = "WORKER"

	METHOD      = "METHOD"
	NAME        = "NAME"
	METADATA    = "METADATA"
	REQUEST     = "REQUEST"
	REPLY       = "REPLY"
	TIME        = "TIME"
	TITILE      = "TITLE"
	STACK_TRACE = "STACK_TRACE"
	CODE        = "CODE"
	STATUS      = "STATUS"
	MSG         = "MSG"
	DETAILS     = "DETAILS"
	ERR         = "ERR"
	TYPE        = "TYPE"
	REQUEST_ID  = "REQUEST_ID"
	URI         = "URI"
	LATENCY     = "LATENCY"
)


// pkg/constant.go
package pkg

const Bearer = "Bearer"


// pkg/error/contracts/contracts.go
package errorContract

import (
	"github.com/pkg/errors"
)

type StackTracer interface {
	StackTrace() errors.StackTrace
}


// pkg/error/custom_error/application_error.go
package customError

import (
	"github.com/pkg/errors"
)

type applicationError struct {
	CustomError
}

func (e *applicationError) IsApplicationError() bool {
	return true
}

type ApplicationError interface {
	CustomError
	IsApplicationError() bool
}

func IsApplicationError(e error) bool {
	var applicationError ApplicationError

	if errors.As(e, &applicationError) {
		return applicationError.IsApplicationError()
	}

	return false
}

func NewApplicationError(message string, code int, details map[string]string) error {
	e := &applicationError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewApplicationErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &applicationError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/bad_request_error.go
package customError

import (
	"github.com/pkg/errors"
)

type badRequestError struct {
	CustomError
}

func (e *badRequestError) IsBadRequestError() bool {
	return true
}

type BadRequestError interface {
	CustomError
	IsBadRequestError() bool
}

func IsBadRequestError(e error) bool {
	var badRequestError BadRequestError

	if errors.As(e, &badRequestError) {
		return badRequestError.IsBadRequestError()
	}

	return false
}

func NewBadRequestError(message string, code int, details map[string]string) error {
	e := &badRequestError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewBadRequestErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &badRequestError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/conflict_error.go
package customError

import (
	"github.com/pkg/errors"
)

type conflictError struct {
	CustomError
}

func (e *conflictError) IsConflictError() bool {
	return true
}

type ConflictError interface {
	CustomError
	IsConflictError() bool
}

func IsConflictError(e error) bool {
	var conflictError ConflictError

	if errors.As(e, &conflictError) {
		return conflictError.IsConflictError()
	}

	return false
}

func NewConflictError(message string, code int, details map[string]string) error {
	e := &conflictError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewConflictErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &conflictError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/custom_error.go
package customError

import "github.com/pkg/errors"

type customError struct {
	internalCode int
	message      string
	err          error
	details      map[string]string
}

func (ce *customError) Error() string {
	if ce.err != nil {
		return ce.message + ": " + ce.err.Error()
	}

	return ce.message
}

func (ce *customError) Message() string {
	return ce.message
}

func (ce *customError) Code() int {
	return ce.internalCode
}

func (ce *customError) Details() map[string]string {
	return ce.details
}

func (ce *customError) IsCustomError() bool {
	return true
}

type CustomError interface {
	error
	IsCustomError() bool
	Message() string
	Code() int
	Details() map[string]string
}

func IsCustomError(err error) bool {
	var customErr CustomError
	if errors.As(err, &customErr) {
		return customErr.IsCustomError()
	}
	return false
}

func NewCustomError(err error, internalCode int, message string, details map[string]string) CustomError {
	ce := &customError{
		internalCode: internalCode,
		err:          err,
		message:      message,
		details:      details,
	}

	return ce
}

func AsCustomError(err error) CustomError {
	var customErr CustomError
	if errors.As(err, &customErr) {
		return customErr
	}
	return nil
}


// pkg/error/custom_error/domain_error.go
package customError

import (
	"github.com/pkg/errors"
)

type domainError struct {
	CustomError
}

func (e *domainError) IsDomainError() bool {
	return true
}

type DomainError interface {
	CustomError
	IsDomainError() bool
}

func IsDomainError(e error) bool {
	var domainError DomainError

	if errors.As(e, &domainError) {
		return domainError.IsDomainError()
	}
	return false
}

func NewDomainError(message string, code int, details map[string]string) error {
	e := &domainError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewDomainErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &domainError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/forbiden_error.go
package customError

import (
	"github.com/pkg/errors"
)

type forbiddenError struct {
	CustomError
}

func (e *forbiddenError) IsForbiddenError() bool {
	return true
}

type ForbiddenError interface {
	CustomError
	IsForbiddenError() bool
}

func IsForbiddenError(e error) bool {
	var forbiddenError ForbiddenError

	if errors.As(e, &forbiddenError) {
		return forbiddenError.IsForbiddenError()
	}

	return false
}

func NewForbiddenError(message string, code int, details map[string]string) error {
	e := &forbiddenError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewForbiddenErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &forbiddenError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/internal_server_error.go
package customError

import (
	"github.com/pkg/errors"
)

type internalServerError struct {
	CustomError
}

func (e *internalServerError) IsInternalServerError() bool {
	return true
}

type InternalServerError interface {
	CustomError
	IsInternalServerError() bool
}

func IsInternalServerError(e error) bool {
	var internalError InternalServerError

	if errors.As(e, &internalError) {
		return internalError.IsInternalServerError()
	}

	return false
}

func NewInternalServerError(message string, code int, details map[string]string) error {
	e := &internalServerError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewInternalServerErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &internalServerError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/marshaling_error.go
package customError

import (
	"github.com/pkg/errors"
)

type marshalingError struct {
	CustomError
}

func (e *marshalingError) IsMarshalingError() bool {
	return true
}

type MarshalingError interface {
	CustomError
	IsMarshalingError() bool
}

func IsMarshalingError(e error) bool {
	var marshalingError MarshalingError

	if errors.As(e, &marshalingError) {
		return marshalingError.IsMarshalingError()
	}

	return false
}

func NewMarshalingError(message string, code int, details map[string]string) error {
	e := &marshalingError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewMarshalingErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &marshalingError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/not_found_error.go
package customError

import (
	"github.com/pkg/errors"
)

type notFoundError struct {
	CustomError
}

func (e *notFoundError) IsNotFoundError() bool {
	return true
}

type NotFoundError interface {
	CustomError
	IsNotFoundError() bool
}

func IsNotFoundError(e error) bool {
	var notFoundError NotFoundError

	if errors.As(e, &notFoundError) {
		return notFoundError.IsNotFoundError()
	}

	return false
}

func NewNotFoundError(message string, code int, details map[string]string) error {
	e := &notFoundError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewNotFoundErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &notFoundError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/unauthorized_error.go
package customError

import (
	"github.com/pkg/errors"
)

type unauthorizedError struct {
	CustomError
}

func (e *unauthorizedError) IsUnAuthorizedError() bool {
	return true
}

type UnauthorizedError interface {
	CustomError
	IsUnAuthorizedError() bool
}

func IsUnAuthorizedError(e error) bool {
	var unauthorizedError UnauthorizedError

	if errors.As(e, &unauthorizedError) {
		return unauthorizedError.IsUnAuthorizedError()
	}

	return false
}

func NewUnAuthorizedError(message string, code int, details map[string]string) error {
	e := &unauthorizedError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewUnAuthorizedErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &unauthorizedError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/unmarshaling_error.go
package customError

import (
	"github.com/pkg/errors"
)

type unMarshalingError struct {
	CustomError
}

func (e *unMarshalingError) IsUnMarshalingError() bool {
	return true
}

type UnMarshalingError interface {
	CustomError
	IsUnMarshalingError() bool
}

func IsUnMarshalingError(e error) bool {
	var unMarshalingError UnMarshalingError

	if errors.As(e, &unMarshalingError) {
		return unMarshalingError.IsUnMarshalingError()
	}

	return false
}

func NewUnMarshalingError(message string, code int, details map[string]string) error {
	e := &unMarshalingError{
		CustomError: NewCustomError(nil, code, message, details),
	}

	return e
}

func NewUnMarshalingErrorWrap(err error, message string, code int, details map[string]string) error {
	e := &unMarshalingError{
		CustomError: NewCustomError(err, code, message, details),
	}
	stackErr := errors.WithStack(e)

	return stackErr
}


// pkg/error/custom_error/validation_error.go
package customError

import (
	"github.com/pkg/errors"
)

type validationError struct {
	BadRequestError
}

func (e *validationError) IsValidationError() bool {
	return true
}

type ValidationError interface {
	BadRequestError
	IsValidationError() bool
}

func IsValidationError(e error) bool {
	var validationError ValidationError

	if errors.As(e, &validationError) {
		return validationError.IsValidationError()
	}

	return false
}

func NewValidationError(message string, code int, details map[string]string) error {
	e := NewBadRequestError(message, code, details)
	ve := &validationError{
		BadRequestError: &badRequestError{
			CustomError: AsCustomError(e),
		},
	}

	return ve
}

func NewValidationErrorWrap(err error, message string, code int, details map[string]string) error {
	e := NewBadRequestErrorWrap(err, message, code, details)
	ve := &validationError{
		BadRequestError: &badRequestError{
			CustomError: AsCustomError(e),
		},
	}

	stackErr := errors.WithStack(ve)

	return stackErr
}


// pkg/error/error_utils/error_utils.go
package errorUtils

import (
	"context"
	"fmt"
	"strings"

	validator "github.com/go-ozzo/ozzo-validation"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	errorList "github.com/diki-haryadi/go-micro-template/pkg/constant/error/error_list"
	errorContract "github.com/diki-haryadi/go-micro-template/pkg/error/contracts"
	customError "github.com/diki-haryadi/go-micro-template/pkg/error/custom_error"
	"github.com/diki-haryadi/ztools/logger"
)

// CheckErrorMessages checks for specific messages contains in the error
func CheckErrorMessages(err error, messages ...string) bool {
	for _, message := range messages {
		if strings.Contains(strings.TrimSpace(strings.ToLower(err.Error())), strings.TrimSpace(strings.ToLower(message))) {
			return true
		}
	}
	return false
}

// RootStackTrace returns root stack trace with a string contains just stack trace levels for the given error
func RootStackTrace(err error) string {
	var stackStr string
	for {
		st, ok := err.(errorContract.StackTracer)
		if ok {
			stackStr = fmt.Sprintf("%+v\n", st.StackTrace())

			if !ok {
				break
			}
		}
		err = errors.Unwrap(err)
		if err == nil {
			break
		}
	}

	return stackStr
}

func ValidationErrorHandler(err error) (map[string]string, error) {
	var customErr validator.Errors
	if errors.As(err, &customErr) {
		details := make(map[string]string)
		for k, v := range customErr {
			details[k] = v.Error()
		}
		return details, nil
	}
	internalServerError := errorList.InternalErrorList.InternalServerError
	return nil, customError.NewInternalServerErrorWrap(err, internalServerError.Msg, internalServerError.Code, nil)
}

type HandlerFunc func() error
type WrappedFunc func()

func HandlerErrorWrapper(ctx context.Context, f HandlerFunc) WrappedFunc { // must return without error
	return func() {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					logger.Zap.Sugar().Errorf("%v", r)
					return
				}
				logger.Zap.Error(err.Error(), zap.Error(err))
			}
		}()
		e := f()
		if e != nil {
			fmt.Println(e)
		}
	}
}


// pkg/error/grpc/custom_grpc_error.go
package grpcError

import (
	"time"

	"google.golang.org/grpc/codes"

	errorConstant "github.com/diki-haryadi/go-micro-template/pkg/constant/error"
)

func NewGrpcValidationError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrValidationFailedTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.InvalidArgument,
		Timestamp: time.Now(),
	}
}

func NewGrpcConflictError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrConflictTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.AlreadyExists,
		Timestamp: time.Now(),
	}
}

func NewGrpcBadRequestError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrBadRequestTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.InvalidArgument,
		Timestamp: time.Now(),
	}
}

func NewGrpcNotFoundError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrNotFoundTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.NotFound,
		Timestamp: time.Now(),
	}
}

func NewGrpcUnAuthorizedError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrUnauthorizedTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.Unauthenticated,
		Timestamp: time.Now(),
	}
}

func NewGrpcForbiddenError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrForbiddenTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.PermissionDenied,
		Timestamp: time.Now(),
	}
}

func NewGrpcInternalServerError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrInternalServerErrorTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.Internal,
		Timestamp: time.Now(),
	}
}

func NewGrpcDomainError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrDomainTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.InvalidArgument,
		Timestamp: time.Now(),
	}
}

func NewGrpcApplicationError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrApplicationTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.Internal,
		Timestamp: time.Now(),
	}
}

func NewGrpcApiError(code int, message string, details map[string]string) GrpcErr {
	return &grpcErr{
		Title:     errorConstant.ErrApiTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    codes.Internal,
		Timestamp: time.Now(),
	}
}


// pkg/error/grpc/grpc_error.go
package grpcError

import (
	"time"

	errorBuf "github.com/diki-haryadi/protobuf-template/shared/error/v1"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcErr struct {
	Status    codes.Code        `json:"status,omitempty"`
	Code      int               `json:"code,omitempty"`
	Title     string            `json:"title,omitempty"`
	Msg       string            `json:"msg,omitempty"`
	Details   map[string]string `json:"errorDetail,omitempty"`
	Timestamp time.Time         `json:"timestamp,omitempty"`
}

type GrpcErr interface {
	GetStatus() codes.Code
	SetStatus(status codes.Code) GrpcErr
	GetCode() int
	SetCode(code int) GrpcErr
	GetTitle() string
	SetTitle(title string) GrpcErr
	GetMsg() string
	SetMsg(msg string) GrpcErr
	GetDetails() map[string]string
	SetDetails(details map[string]string) GrpcErr
	GetTimestamp() time.Time
	SetTimestamp(time time.Time) GrpcErr
	Error() string
	ErrBody() error
	ToGrpcResponseErr() error
}

func NewGrpcError(status codes.Code, code int, title string, message string, details map[string]string) GrpcErr {
	grpcErr := &grpcErr{
		Status:    status,
		Code:      code,
		Title:     title,
		Msg:       message,
		Details:   details,
		Timestamp: time.Now(),
	}

	return grpcErr
}

func (ge *grpcErr) ErrBody() error {
	return ge
}

func (ge *grpcErr) Error() string {
	return ge.Msg
}

func (ge *grpcErr) GetStatus() codes.Code {
	return ge.Status
}

func (ge *grpcErr) SetStatus(status codes.Code) GrpcErr {
	ge.Status = status

	return ge
}

func (ge *grpcErr) GetCode() int {
	return ge.Code
}

func (ge *grpcErr) SetCode(code int) GrpcErr {
	ge.Code = code

	return ge
}

func (ge *grpcErr) GetTitle() string {
	return ge.Title
}

func (ge *grpcErr) SetTitle(title string) GrpcErr {
	ge.Title = title

	return ge
}

func (ge *grpcErr) GetMsg() string {
	return ge.Msg
}

func (ge *grpcErr) SetMsg(message string) GrpcErr {
	ge.Msg = message

	return ge
}

func (ge *grpcErr) GetDetails() map[string]string {
	return ge.Details
}

func (ge *grpcErr) SetDetails(detail map[string]string) GrpcErr {
	ge.Details = detail

	return ge
}

func (ge *grpcErr) GetTimestamp() time.Time {
	return ge.Timestamp
}

func (ge *grpcErr) SetTimestamp(time time.Time) GrpcErr {
	ge.Timestamp = time

	return ge
}

func IsGrpcError(err error) bool {
	var grpcErr GrpcErr
	return errors.As(err, &grpcErr)
}

// ToGrpcResponseErr creates a gRPC error response to send grpc engine
func (ge *grpcErr) ToGrpcResponseErr() error {
	st := status.New(ge.Status, ge.Error())
	mappedErr := &errorBuf.CustomError{
		Title:     ge.Title,
		Code:      int64(ge.Code),
		Msg:       ge.Msg,
		Details:   ge.Details,
		Timestamp: ge.Timestamp.Format(time.RFC3339),
	}
	stWithDetails, _ := st.WithDetails(mappedErr)
	return stWithDetails.Err()
}

func ParseExternalGrpcErr(err error) GrpcErr {
	st := status.Convert(err)
	for _, detail := range st.Details() {
		if t, ok := detail.(*errorBuf.CustomError); ok {
			timestamp, _ := time.Parse(time.RFC3339, t.Timestamp)
			return &grpcErr{
				Status:    st.Code(),
				Code:      int(t.Code),
				Title:     t.Title,
				Msg:       t.Msg,
				Details:   t.Details,
				Timestamp: timestamp,
			}
		}
	}
	return nil
}


// pkg/error/grpc/grpc_error_parser.go
package grpcError

import (
	"google.golang.org/grpc/codes"

	errorList "github.com/diki-haryadi/go-micro-template/pkg/constant/error/error_list"
	customError "github.com/diki-haryadi/go-micro-template/pkg/error/custom_error"
)

func ParseError(err error) GrpcErr {
	customErr := customError.AsCustomError(err)
	if customErr == nil {
		internalServerError := errorList.InternalErrorList.InternalServerError
		err =
			customError.NewInternalServerErrorWrap(err, internalServerError.Msg, internalServerError.Code, nil)
		customErr = customError.AsCustomError(err)
		return NewGrpcError(codes.Internal, customErr.Code(), codes.Internal.String(), customErr.Error(), customErr.Details())
	}

	if err != nil {
		switch {
		case customError.IsValidationError(err):
			return NewGrpcValidationError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsBadRequestError(err):
			return NewGrpcBadRequestError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsNotFoundError(err):
			return NewGrpcNotFoundError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsInternalServerError(err):
			return NewGrpcInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsForbiddenError(err):
			return NewGrpcForbiddenError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsUnAuthorizedError(err):
			return NewGrpcUnAuthorizedError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsDomainError(err):
			return NewGrpcDomainError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsApplicationError(err):
			return NewGrpcApplicationError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsConflictError(err):
			return NewGrpcConflictError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsUnMarshalingError(err):
			return NewGrpcInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsMarshalingError(err):
			return NewGrpcInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsCustomError(err):
			return NewGrpcError(codes.Internal, customErr.Code(), codes.Internal.String(), customErr.Message(), customErr.Details())

		// case error.Is(err, context.DeadlineExceeded):
		// 	return NewGrpcError(codes.DeadlineExceeded, customErr.Code(), errorTitles.ErrRequestTimeoutTitle, err.Error(), stackTrace)

		default:
			return NewGrpcInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())
		}
	}

	return nil
}


// pkg/error/http/custom_http_error.go
package httpError

import (
	"net/http"
	"time"

	errorConstant "github.com/diki-haryadi/go-micro-template/pkg/constant/error"
)

func NewHttpValidationError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrValidationFailedTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusBadRequest,
		Timestamp: time.Now(),
	}
}

func NewHttpConflictError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrConflictTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusConflict,
		Timestamp: time.Now(),
	}
}

func NewHttpBadRequestError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrBadRequestTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusBadRequest,
		Timestamp: time.Now(),
	}
}

func NewHttpNotFoundError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrNotFoundTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusNotFound,
		Timestamp: time.Now(),
	}
}

func NewHttpUnAuthorizedError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrUnauthorizedTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusUnauthorized,
		Timestamp: time.Now(),
	}
}

func NewHttpForbiddenError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrForbiddenTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusForbidden,
		Timestamp: time.Now(),
	}
}

func NewHttpInternalServerError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrInternalServerErrorTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusInternalServerError,
		Timestamp: time.Now(),
	}
}

func NewHttpDomainError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrDomainTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusBadRequest,
		Timestamp: time.Now(),
	}
}

func NewHttpApplicationError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrApplicationTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusInternalServerError,
		Timestamp: time.Now(),
	}
}

func NewHttpApiError(code int, message string, details map[string]string) HttpErr {
	return &httpErr{
		Title:     errorConstant.ErrApiTitle,
		Code:      code,
		Msg:       message,
		Details:   details,
		Status:    http.StatusInternalServerError,
		Timestamp: time.Now(),
	}
}


// pkg/error/http/http_error.go
package httpError

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

type httpErr struct {
	Status    int               `json:"status,omitempty"`
	Code      int               `json:"code,omitempty"`
	Title     string            `json:"title,omitempty"`
	Msg       string            `json:"msg,omitempty"`
	Details   map[string]string `json:"errorDetail,omitempty"`
	Timestamp time.Time         `json:"timestamp,omitempty"`
}

type HttpErr interface {
	GetStatus() int
	SetStatus(status int) HttpErr
	GetCode() int
	SetCode(code int) HttpErr
	GetTitle() string
	SetTitle(title string) HttpErr
	GetMsg() string
	SetMsg(msg string) HttpErr
	GetDetails() map[string]string
	SetDetails(details map[string]string) HttpErr
	GetTimestamp() time.Time
	SetTimestamp(time time.Time) HttpErr
	Error() string
	ErrBody() error
	WriteTo(w http.ResponseWriter) (int, error)
	// ToGrpcResponseErr() error
}

func NewHttpError(status int, code int, title string, messahe string, details map[string]string) HttpErr {
	httpErr := &httpErr{
		Status:    status,
		Code:      code,
		Title:     title,
		Msg:       messahe,
		Details:   details,
		Timestamp: time.Now(),
	}

	return httpErr
}

func (he *httpErr) ErrBody() error {
	return he
}

func (he *httpErr) Error() string {
	return he.Msg
}

func (he *httpErr) GetStatus() int {
	return he.Status
}

func (he *httpErr) SetStatus(status int) HttpErr {
	he.Status = status

	return he
}

func (he *httpErr) GetCode() int {
	return he.Code
}

func (he *httpErr) SetCode(code int) HttpErr {
	he.Code = code

	return he
}

func (he *httpErr) GetTitle() string {
	return he.Title
}

func (he *httpErr) SetTitle(title string) HttpErr {
	he.Title = title

	return he
}

func (he *httpErr) GetMsg() string {
	return he.Msg
}

func (he *httpErr) SetMsg(messahe string) HttpErr {
	he.Msg = messahe

	return he
}

func (he *httpErr) GetDetails() map[string]string {
	return he.Details
}

func (he *httpErr) SetDetails(detail map[string]string) HttpErr {
	he.Details = detail

	return he
}

func (he *httpErr) GetTimestamp() time.Time {
	return he.Timestamp
}

func (he *httpErr) SetTimestamp(time time.Time) HttpErr {
	he.Timestamp = time

	return he
}

func IsHttpError(err error) bool {
	var httpErr HttpErr

	return errors.As(err, &httpErr)
}

const (
	ContentTypeJSON = "application/problem+json"
)

// WriteTo writes the JSON Problem to an HTTP Response Writer
func (he *httpErr) WriteTo(w http.ResponseWriter) (int, error) {
	status := he.GetStatus()
	if status == 0 {
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)

	return w.Write(he.json())
}

func (he *httpErr) json() []byte {
	b, _ := json.Marshal(&he)

	return b
}

// Don't forget to clese the body : <defer body.Close()>

func ParseExternalHttpErr(body io.ReadCloser) HttpErr {
	he := new(httpErr)
	_ = json.NewDecoder(body).Decode(he)

	return he
}


// pkg/error/http/http_error_parser.go
package httpError

import (
	"net/http"

	"google.golang.org/grpc/codes"

	errorConstant "github.com/diki-haryadi/go-micro-template/pkg/constant/error"
	errorList "github.com/diki-haryadi/go-micro-template/pkg/constant/error/error_list"
	customError "github.com/diki-haryadi/go-micro-template/pkg/error/custom_error"
)

func ParseError(err error) HttpErr {
	customErr := customError.AsCustomError(err)
	if customErr == nil {
		internalServerError := errorList.InternalErrorList.InternalServerError
		err =
			customError.NewInternalServerErrorWrap(err, internalServerError.Msg, internalServerError.Code, nil)
		customErr = customError.AsCustomError(err)
		return NewHttpError(http.StatusInternalServerError, customErr.Code(), errorConstant.ErrInternalServerErrorTitle, customErr.Error(), customErr.Details())
	}

	if err != nil {
		switch {
		case customError.IsValidationError(err):
			return NewHttpValidationError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsBadRequestError(err):
			return NewHttpBadRequestError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsNotFoundError(err):
			return NewHttpNotFoundError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsInternalServerError(err):
			return NewHttpInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsForbiddenError(err):
			return NewHttpForbiddenError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsUnAuthorizedError(err):
			return NewHttpUnAuthorizedError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsDomainError(err):
			return NewHttpDomainError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsApplicationError(err):
			return NewHttpApplicationError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsConflictError(err):
			return NewHttpConflictError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsUnMarshalingError(err):
			return NewHttpInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsMarshalingError(err):
			return NewHttpInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())

		case customError.IsCustomError(err):
			return NewHttpError(http.StatusInternalServerError, customErr.Code(), codes.Internal.String(), customErr.Message(), customErr.Details())

		// case error.Is(err, context.DeadlineExceeded):
		// 	return NewHttpError(codes.DeadlineExceeded, customErr.Code(), errorTitles.ErrRequestTimeoutTitle, err.Error(), stackTrace)

		default:
			return NewHttpInternalServerError(customErr.Code(), customErr.Message(), customErr.Details())
		}
	}

	return nil
}


// pkg/password.go
package pkg

import (
	"golang.org/x/crypto/bcrypt"
)

// VerifyPassword compares password and the hashed password
func VerifyPassword(passwordHash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

// HashPassword creates a bcrypt password hash
func HashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), 3)
}


// pkg/response/error.go
package response

import (
	"encoding/json"
	"fmt"
)

type ErrChain struct {
	Message string
	Cause   error
	Fields  map[string]interface{}
	Type    error
}

func (err ErrChain) Error() string {
	bcoz := ""
	fields := ""
	if err.Cause != nil {
		bcoz = fmt.Sprint(" because {", err.Cause.Error(), "}")
		if len(err.Fields) > 0 {
			fields = fmt.Sprintf(" with Fields {%+v}", err.Fields)
		}
	}
	return fmt.Sprint(err.Message, bcoz, fields)
}

func Type(err error) error {
	switch err.(type) {
	case ErrChain:
		return err.(ErrChain).Type
	}
	return nil
}

func toString(m map[string]string) string {
	v, _ := json.Marshal(m)
	return string(v)
}

func (err ErrChain) SetField(key string, value string) ErrChain {
	if err.Fields == nil {
		err.Fields = map[string]interface{}{}
	}
	err.Fields[key] = value
	return err
}

type InvalidError struct {
	message string
}

func (ie *InvalidError) Error() string {
	return ie.message
}

func NewInvalidError(msg string) *InvalidError {
	return &InvalidError{message: msg}
}

func NewInvalidErrorf(msg string, args ...interface{}) *InvalidError {
	return NewInvalidError(fmt.Sprintf(msg, args...))
}


// pkg/response/response.go
package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diki-haryadi/go-micro-template/pkg/constant"
	"log"
	"net/http"
	"strconv"
)

var (
	ErrBadRequest                    = errors.New("Bad request")
	ErrForbiddenResource             = errors.New("Forbidden resource")
	ErrNotFound                      = errors.New("Not Found")
	ErrPreConditionFailed            = errors.New("Precondition failed")
	ErrInternalServerError           = errors.New("Internal server error")
	ErrTimeoutError                  = errors.New("Timeout error")
	ErrUnauthorized                  = errors.New("Unauthorized")
	ErrConflict                      = errors.New("Conflict")
	ErrMethodNotAllowed              = errors.New("Method not allowed")
	ErrInvalidGrantType              = errors.New("Invalid grant type")
	ErrInvalidClientIDOrSecret       = errors.New("Invalid client ID or secret")
	ErrAuthorizationCodeNotFound     = errors.New("Authorization code not found")
	ErrAuthorizationCodeExpired      = errors.New("Authorization code expired")
	ErrInvalidRedirectURI            = errors.New("Invalid redirect URI")
	ErrInvalidScope                  = errors.New("Invalid scope")
	ErrInvalidUsernameOrPassword     = errors.New("Invalid username or password")
	ErrInvalidPassword               = errors.New("Invalid password")
	ErrInvalidPasswordCannotSame     = errors.New("Invalid password cannot same")
	ErrRefreshTokenNotFound          = errors.New("Refresh token not found")
	ErrRefreshTokenExpired           = errors.New("Refresh token expired")
	ErrRequestedScopeCannotBeGreater = errors.New("Requested scope cannot be greater")
	ErrTokenMissing                  = errors.New("Username missing")
	ErrTokenHintInvalid              = errors.New("Invalid token hint")
	ErrAccessTokenNotFound           = errors.New("Access token not found")
	ErrAccessTokenExpired            = errors.New("Access token expired")
	ErrClientNotFound                = errors.New("Client not found")
	ErrInvalidClientSecret           = errors.New("Invalid client secret")
	ErrClientIDTaken                 = errors.New("Client ID taken")
	ErrRoleNotFound                  = errors.New("Role not found")
	MinPasswordLength                = 6
	ErrPasswordTooShort              = fmt.Errorf(
		"Password must be at least %d characters long",
		MinPasswordLength,
	)
	ErrUserNotFound                         = errors.New("User not found")
	ErrInvalidUserPassword                  = errors.New("Invalid user password")
	ErrCannotSetEmptyUsername               = errors.New("Cannot set empty username")
	ErrUserPasswordNotSet                   = errors.New("User password not set")
	ErrUsernameTaken                        = errors.New("Username taken")
	ErrInvalidAuthorizationCodeGrantRequest = errors.New("Invalid authorization code request")
	ErrInvalidPasswordGrantRequest          = errors.New("Invalid password grant request")
	ErrInvalidClientCredentialsGrantRequest = errors.New("Invalid client credentials grant request")
	ErrInvalidIntrospectRequest             = errors.New("Invalid introspect request")
	ErrSessonNotStarted                     = errors.New("Session not started")
)

const (
	StatusCodeGenericSuccess            = "200000"
	StatusCodeAccepted                  = "202000"
	StatusCodeBadRequest                = "400000"
	StatusCodeAlreadyRegistered         = "400001"
	StatusCodeUnauthorized              = "401000"
	StatusCodeForbidden                 = "403000"
	StatusCodeNotFound                  = "404000"
	StatusCodeConflict                  = "409000"
	StatusCodeGenericPreconditionFailed = "412000"
	StatusCodeOTPLimitReached           = "412550"
	StatusCodeNoLinkerExist             = "412553"
	StatusCodeInternalError             = "500000"
	StatusCodeFailedSellBatch           = "500100"
	StatusCodeFailedOTP                 = "503000"
	StatusCodeServiceUnavailable        = "503000"
	StatusCodeTimeoutError              = "504000"
	StatusCodeMethodNotAllowed          = "405000"
)

func GetErrorCode(err error) string {
	err = getErrType(err)

	switch err {
	case ErrBadRequest:
		return StatusCodeBadRequest
	case ErrForbiddenResource:
		return StatusCodeForbidden
	case ErrNotFound:
		return StatusCodeNotFound
	case ErrPreConditionFailed:
		return StatusCodeGenericPreconditionFailed
	case ErrInternalServerError:
		return StatusCodeInternalError
	case ErrTimeoutError:
		return StatusCodeTimeoutError
	case ErrUnauthorized:
		return StatusCodeUnauthorized
	case ErrConflict:
		return StatusCodeConflict
	case ErrMethodNotAllowed:
		return StatusCodeMethodNotAllowed
	case ErrInvalidGrantType, ErrInvalidClientIDOrSecret, ErrInvalidRedirectURI, ErrInvalidScope,
		ErrInvalidUsernameOrPassword, ErrInvalidPassword, ErrInvalidPasswordCannotSame, ErrTokenHintInvalid, ErrInvalidAuthorizationCodeGrantRequest,
		ErrInvalidPasswordGrantRequest, ErrInvalidClientCredentialsGrantRequest, ErrInvalidIntrospectRequest:
		return StatusCodeBadRequest
	case ErrAuthorizationCodeNotFound, ErrRefreshTokenNotFound, ErrTokenMissing, ErrAccessTokenNotFound:
		return StatusCodeNotFound
	case ErrAuthorizationCodeExpired, ErrRefreshTokenExpired, ErrRequestedScopeCannotBeGreater, ErrAccessTokenExpired:
		return StatusCodeBadRequest
	case ErrClientNotFound, ErrInvalidClientSecret, ErrUserNotFound, ErrRoleNotFound:
		return StatusCodeNotFound
	case ErrClientIDTaken, ErrUsernameTaken:
		return StatusCodeConflict
	case ErrPasswordTooShort, ErrCannotSetEmptyUsername, ErrUserPasswordNotSet, ErrInvalidUserPassword:
		return StatusCodeBadRequest
	case nil:
		return StatusCodeGenericSuccess
	default:
		return StatusCodeInternalError
	}
}

func GetHTTPStatus(code int) string {
	switch code {
	case http.StatusOK:
		return "success"
	case http.StatusCreated:
		return "created"
	case http.StatusAccepted:
		return "accepted"
	case http.StatusNonAuthoritativeInfo:
		return "non authoritative information"
	case http.StatusNoContent:
		return "no content"
	case http.StatusResetContent:
		return "reset content"
	case http.StatusPartialContent:
		return "partial content"
	case http.StatusMultipleChoices:
		return "multiple choices"
	case http.StatusMovedPermanently:
		return "moved permanently"
	case http.StatusFound:
		return "found"
	case http.StatusSeeOther:
		return "see other"
	case http.StatusNotModified:
		return "not modified"
	case http.StatusUseProxy:
		return "use proxy"
	case http.StatusTemporaryRedirect:
		return "temporary redirect"
	case http.StatusPermanentRedirect:
		return "permanent redirect"
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusPaymentRequired:
		return "payment required"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not found"
	case http.StatusMethodNotAllowed:
		return "method not allowed"
	case http.StatusNotAcceptable:
		return "not acceptable"
	case http.StatusProxyAuthRequired:
		return "proxy authentication required"
	case http.StatusRequestTimeout:
		return "request timeout"
	case http.StatusConflict:
		return "conflict"
	case http.StatusGone:
		return "gone"
	case http.StatusLengthRequired:
		return "length required"
	case http.StatusPreconditionFailed:
		return "precondition failed"
	case http.StatusRequestEntityTooLarge:
		return "request entity too large"
	case http.StatusRequestURITooLong:
		return "request URI too long"
	case http.StatusUnsupportedMediaType:
		return "unsupported media type"
	case http.StatusRequestedRangeNotSatisfiable:
		return "requested range not satisfiable"
	case http.StatusExpectationFailed:
		return "expectation failed"
	case http.StatusTeapot:
		return "I'm a teapot"
	case http.StatusMisdirectedRequest:
		return "misdirected request"
	case http.StatusUnprocessableEntity:
		return "unprocessable entity"
	case http.StatusLocked:
		return "locked"
	case http.StatusFailedDependency:
		return "failed dependency"
	case http.StatusUpgradeRequired:
		return "upgrade required"
	case http.StatusPreconditionRequired:
		return "precondition required"
	case http.StatusTooManyRequests:
		return "too many requests"
	case http.StatusRequestHeaderFieldsTooLarge:
		return "request header fields too large"
	case http.StatusUnavailableForLegalReasons:
		return "unavailable for legal reasons"
	case http.StatusInternalServerError:
		return "internal server error"
	case http.StatusNotImplemented:
		return "not implemented"
	case http.StatusBadGateway:
		return "bad gateway"
	case http.StatusServiceUnavailable:
		return "service unavailable"
	case http.StatusGatewayTimeout:
		return "gateway timeout"
	case http.StatusHTTPVersionNotSupported:
		return "HTTP version not supported"
	case http.StatusVariantAlsoNegotiates:
		return "variant also negotiates"
	case http.StatusInsufficientStorage:
		return "insufficient storage"
	case http.StatusLoopDetected:
		return "loop detected"
	case http.StatusNotExtended:
		return "not extended"
	case http.StatusNetworkAuthenticationRequired:
		return "network authentication required"
	default:
		return "undefined"
	}
}

func GetHTTPCode(code string) int {
	s := code[0:3]
	i, _ := strconv.Atoi(s)
	return i
}

type JSONResponse struct {
	Data        interface{}            `json:"data,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Code        string                 `json:"code"`
	StatusCode  int                    `json:"status_code"`
	Status      string                 `json:"status"`
	ErrorString string                 `json:"error,omitempty"`
	Error       error                  `json:"-"`
	RealError   string                 `json:"-"`
	Latency     string                 `json:"latency,omitempty"`
	Log         map[string]interface{} `json:"-"`
	HTMLPage    bool                   `json:"-"`
	Result      interface{}            `json:"result,omitempty"`
}

func NewJSONResponse() *JSONResponse {
	return &JSONResponse{Code: StatusCodeGenericSuccess, StatusCode: GetHTTPCode(StatusCodeGenericSuccess), Status: GetHTTPStatus(http.StatusOK), Log: map[string]interface{}{}}
}

func (r *JSONResponse) SetData(data interface{}) *JSONResponse {
	r.Data = data
	return r
}

func (r *JSONResponse) SetStatus(status string) *JSONResponse {
	r.Status = status
	return r
}

func (r *JSONResponse) SetCode(code string) *JSONResponse {
	r.Code = code
	return r
}

func (r *JSONResponse) SetStatusCode(statusCode int) *JSONResponse {
	r.StatusCode = statusCode
	return r
}

func (r *JSONResponse) SetHTML() *JSONResponse {
	r.HTMLPage = true
	return r
}

func (r *JSONResponse) SetResult(result interface{}) *JSONResponse {
	r.Result = result
	return r
}

func (r *JSONResponse) SetMessage(msg string) *JSONResponse {
	r.Message = msg
	return r
}

func (r *JSONResponse) SetLatency(latency float64) *JSONResponse {
	r.Latency = fmt.Sprintf("%.2f ms", latency)
	return r
}

//func (r *JSONResponse) SetLog(key string, val interface{}) *JSONResponse {
//	_, file, no, _ := runtime.Caller(1)
//	log.Errorln(log.Fields{
//		"code":            r.Code,
//		"err":             val,
//		"function_caller": fmt.Sprintf("file %v line no %v", file, no),
//	}).Errorln("Error API")
//	r.Log[key] = val
//	return r
//}

func getErrType(err error) error {
	switch err.(type) {
	case ErrChain:
		errType := err.(ErrChain).Type
		if errType != nil {
			err = errType
		}
	}
	return err
}

func (r *JSONResponse) SetError(err error, a ...string) *JSONResponse {
	r.Code = GetErrorCode(err)
	// r.SetLog("error", err)
	r.RealError = fmt.Sprintf("%+v", err)
	err = getErrType(err)
	r.Error = err
	r.ErrorString = err.Error()
	r.StatusCode = GetHTTPCode(r.Code)
	r.Status = GetHTTPStatus(r.StatusCode)

	if r.StatusCode == http.StatusInternalServerError {
		r.ErrorString = "Internal Server error"
	}
	if len(a) > 0 {
		r.ErrorString = a[0]
	}
	return r
}

func (r *JSONResponse) GetBody() []byte {
	b, _ := json.Marshal(r)
	return b
}

func (r *JSONResponse) Send(w http.ResponseWriter) {
	if r.HTMLPage {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(r.StatusCode)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(r.StatusCode)
		err := json.NewEncoder(w).Encode(r)
		if err != nil {
			log.Println("err", err.Error())
		}
	}
}

// APIStatusSuccess for standard request api status success
func (r *JSONResponse) APIStatusSuccess() *JSONResponse {
	r.Code = constant.StatusCode(constant.StatusSuccess)
	r.Message = constant.StatusText(constant.StatusSuccess)
	return r
}

// APIStatusCreated
func (r *JSONResponse) APIStatusCreated() *JSONResponse {
	r.StatusCode = constant.StatusCreated
	r.Code = constant.StatusCode(constant.StatusCreated)
	r.Message = constant.StatusText(constant.StatusCreated)
	return r
}

// APIStatusAccepted
func (r *JSONResponse) APIStatusAccepted() *JSONResponse {
	r.StatusCode = constant.StatusAccepted
	r.Code = constant.StatusCode(constant.StatusAccepted)
	r.Message = constant.StatusText(constant.StatusAccepted)
	return r
}

// APIStatusNoContent
func (r *JSONResponse) APIStatusNoContent() *JSONResponse {
	r.StatusCode = constant.StatusNoContent
	r.Code = constant.StatusCode(constant.StatusNoContent)
	r.Message = constant.StatusText(constant.StatusNoContent)
	return r
}

// APIStatusErrorUnknown
func (r *JSONResponse) APIStatusErrorUnknown() *JSONResponse {
	r.StatusCode = constant.StatusErrorUnknown
	r.Code = constant.StatusCode(constant.StatusErrorUnknown)
	r.Message = constant.StatusText(constant.StatusErrorUnknown)
	return r
}

// APIStatusInvalidAuthentication
func (r *JSONResponse) APIStatusInvalidAuthentication() *JSONResponse {
	r.StatusCode = constant.StatusInvalidAuthentication
	r.Code = constant.StatusCode(constant.StatusInvalidAuthentication)
	r.Message = constant.StatusText(constant.StatusInvalidAuthentication)
	return r
}

// APIStatusUnauthorized
func (r *JSONResponse) APIStatusUnauthorized() *JSONResponse {
	r.StatusCode = constant.StatusUnauthorized
	r.Code = constant.StatusCode(constant.StatusUnauthorized)
	r.Message = constant.StatusText(constant.StatusUnauthorized)
	return r
}

// APIStatusForbidden
func (r *JSONResponse) APIStatusForbidden() *JSONResponse {
	r.StatusCode = constant.StatusForbidden
	r.Code = constant.StatusCode(constant.StatusForbidden)
	r.Message = constant.StatusText(constant.StatusForbidden)
	return r
}

// APIStatusBadRequest
func (r *JSONResponse) APIStatusBadRequest() *JSONResponse {
	r.StatusCode = constant.StatusErrorForm
	r.Code = constant.StatusCode(constant.StatusErrorForm)
	r.Message = constant.StatusText(constant.StatusErrorForm)
	return r
}

// APIStatusNotFound
func (r *JSONResponse) APIStatusNotFound() *JSONResponse {
	r.StatusCode = constant.StatusNotFound
	r.Code = constant.StatusCode(constant.StatusNotFound)
	r.Message = constant.StatusText(constant.StatusNotFound)
	return r
}


// pkg/sql.go
package pkg

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// IntOrNull returns properly configured sql.NullInt64
func IntOrNull(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: true}
}

// PositiveIntOrNull returns properly configured sql.NullInt64 for a positive number
func PositiveIntOrNull(n int64) sql.NullInt64 {
	if n <= 0 {
		return sql.NullInt64{Int64: 0, Valid: false}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

// FloatOrNull returns properly configured sql.NullFloat64
func FloatOrNull(n float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: n, Valid: true}
}

// PositiveFloatOrNull returns properly configured sql.NullFloat64 for a positive number
func PositiveFloatOrNull(n float64) sql.NullFloat64 {
	if n <= 0 {
		return sql.NullFloat64{Float64: 0.0, Valid: false}
	}
	return sql.NullFloat64{Float64: n, Valid: true}
}

// StringOrNull returns properly configured sql.NullString
func StringOrNull(str string) sql.NullString {
	if str == "" {
		return sql.NullString{String: "", Valid: false}
	}
	return sql.NullString{String: str, Valid: true}
}

// TimeOrNull returns properly confiigured pq.TimeNull
func TimeOrNull(t *time.Time) pq.NullTime {
	if t == nil {
		return pq.NullTime{Time: time.Time{}, Valid: false}
	}
	return pq.NullTime{Time: *t, Valid: true}
}


// pkg/string.go
package pkg

import (
	"strings"
)

// StringInSlice is a function similar to "x in y" Python construct
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// SpaceDelimitedStringNotGreater returns true if the first string
// is the same as the second string or does not contain any substring
// not contained in the second string (when split by space)
func SpaceDelimitedStringNotGreater(first, second string) bool {
	// Empty string is never greater
	if first == "" {
		return true
	}

	// Split the second string by space
	secondParts := strings.Split(second, " ")

	// Iterate over space delimited parts of the first string
	for _, firstPart := range strings.Split(first, " ") {
		// If the substring is not part of the second string, return false
		if !StringInSlice(firstPart, secondParts) {
			return false
		}
	}

	// The first string is the same or more restrictive
	// than the second string, return true
	return true
}


// pkg/utils.go
package pkg


