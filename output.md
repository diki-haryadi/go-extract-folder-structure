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
DIRC      Ûg‹~¢ròõg‹~¢ròõ   ' î'  ¤  è  è  M75é¿æÍø°«kâüÒ1€„> 
.gitignore        g‹~¢ròõg‹~¢ròõ   ' î(  ¤  è  è  D¡®%üAúmóuâ‚¤~8úÓë .pre-commit-config.yaml   g‹~¢ròõg‹~¢ròõ   ' î)  ¤  è  è  -`ÕªC_ÎÄd"~Eğ}îÊ„pò LICENSE   g‹~¢ròõg‹~¢ròõ   ' î*  ¤  è  è  Zéß¢OµV˜{ º”ZázPS« Makefile  g‹~¢ròõg‹~¢ròõ   ' î+  ¤  è  è  à'c;€â¶ØAlæãÆí´“n[j 	README.md g‹~¢ròõg‹~¢ròõ   ' î-  ¤  è  è  û‘¾±C ½ß$ÊŒ ÿS¦‚{ 
app/app.go        g‹~¢ròõg‹~¢ròõ   ' î/  ¤  è  è  
Èóæ¤¡n½É)¢TËG^:ˆÍ cmd/load_data.go  g‹~¢ròõg‹~¢ròõ   ' î0  ¤  è  è  gÄhí3é·S ÓE “‡*( cmd/root.go       g‹~¢ròõg‹~¢ròõ   ' î1  ¤  è  è  í(ÿö~.©cfµ3¯T®±fŞ cmd/serve.go      g‹~¢ròõg‹~¢ròõ   ' î3  ¤  è  è  ¸XNY§IôİÔm¢Œ¬8œTÓ config/config.go  g‹~¢ròõg‹~¢ròõ   ' î6  ¤  è  è  ¦ºÅÉQo ÍÂ;&©ÕbÔ>íá¬G db/fixtures/roles.yml     g‹~¢ròõg‹~¢ròõ   ' î7  ¤  è  è  î±îĞUU€ÕH½5kNlŸñlû‚› db/fixtures/scopes.yml    g‹~¢ròõg‹~¢ròõ   ' î8  ¤  è  è  È]Ó›CRµÆ.3<Äg½¹øü”ö "db/fixtures/test_access_tokens.yml        g‹~¢‚55g‹~¢‚55   ' î9  ¤  è  è  ½v®‹ ùéšhóÚ7ôşû­€tÒ. db/fixtures/test_clients.yml      g‹~¢‚55g‹~¢‚55   ' î:  ¤  è  è  ÑØº{»‡cUV…˜edÔ –­íYÜ db/fixtures/test_users.yml        g‹~¢‚55g‹~¢‚55   ' î<  ¤  è  è    æâ›²ÑÖCK‹)®wZØÂäŒS‘ 2db/migrations/20221110221143_migrate_name.down.sql        g‹~¢‚55g‹~¢‚55   ' î=  ¤  è  è   §ÁÜLÈRÁ#î/T•®û“2 0db/migrations/20221110221143_migrate_name.up.sql  g‹~¢‚55g‹~¢‚55   ' î>  ¤  è  è   Ìd}!ém/Á+À÷ü°‰ÿ³bŸ +db/migrations/20240908110637_users.down.sql       g‹~¢‚55g‹~¢‚55   ' î?  ¤  è  è  ÷èZ
ÊÂ8ËF>Úfñ~†e )db/migrations/20240908110637_users.up.sql g‹~¢‚55g‹~¢‚55   ' î@  ¤  è  è   -Â›¸8”’eÊ¸M>rà&íHñ -db/migrations/20241003072848_clients.down.sql     g‹~¢‚55g‹~¢‚55   ' îA  ¤  è  è  İ¨1î¯FÒàá‰Ì	å})æ´ª,§ +db/migrations/20241003072848_clients.up.sql       g‹~¢‚55g‹~¢‚55   ' îB  ¤  è  è   ğQÆ_A—ØÂîîágµ«µã ,db/migrations/20241003072908_scopes.down.sql      g‹~¢‚55g‹~¢‚55   ' îC  ¤  è  è  Hõ7®İM1÷[<…UÌmİë'Š *db/migrations/20241003072908_scopes.up.sql        g‹~¢‚55g‹~¢‚55   ' îD  ¤  è  è   “Ü-÷º*\½»^õÍrÌ, +db/migrations/20241003072922_roles.down.sql       g‹~¢‚55g‹~¢‚55   ' îE  ¤  è  è  C/ó?ÁúÙ}•İ´#MĞ~ïNDÙ )db/migrations/20241003072922_roles.up.sql g‹~¢‚55g‹~¢‚55   ' îF  ¤  è  è   >ÿIx\h0!İÕIhO¡Ãæ 4db/migrations/20241003072940_refresh_tokens.down.sql      g‹~¢‚55g‹~¢‚55   ' îG  ¤  è  è  «ûOËŸÆ{ŒÁÃŒ~™u! 2db/migrations/20241003072940_refresh_tokens.up.sql        g‹~¢‚55g‹~¢‚55   ' îH  ¤  è  è   Î&@0‘Eç'µ3Û¡éàp5Õÿ 3db/migrations/20241003072953_access_tokens.down.sql       g‹~¢‚55g‹~¢‚55   ' îI  ¤  è  è   ÌéN˜¿YlÔì‹g%@t¡tü§I¾ 1db/migrations/20241003072953_access_tokens.up.sql g‹~¢‚55g‹~¢‚55   ' îJ  ¤  è  è    ”ë±pşxHQKú=EÕ¡ÛÖUµ“ 9db/migrations/20241003073005_authorization_codes.down.sql g‹~¢‚55g‹~¢‚55   ' îK  ¤  è  è  ƒÇ4¥5YúkĞğÇS'=³´€*‡ 7db/migrations/20241003073005_authorization_codes.up.sql   g‹~¢‚55g‹~¢‚55   ' îM  ¤  è  è  KÃO2ns
eïM-›	:FWss˜ deployments/cassandra.yml g‹~¢‚55g‹~¢‚55   ' îN  ¤  è  è  ˆXô¸û††:„5ÚµL>a6•Ë )deployments/docker-compose.e2e-local.yaml g‹~¢‚55g‹~¢‚55   ' îO  ¤  è  è  gDzŞVæ®Cİßs3 ÆqW‰3º deployments/docker-compose.yaml   g‹~¢‚55g‹~¢‚55   ' îQ  ¤  è  è  
ÖÔ°™ß<W˜jÃùîód'd2 docs/admin.http   g‹~¢‚55g‹~¢‚55   ' îS  ¤  è  è  I˜ç3İ÷2,Jˆ\—-ò¶rü-Kë ,docs/api-specification/authorization_code.md      g‹~¢‚55g‹~¢‚55   ' îT  ¤  è  è  ÔâÕH?\€+®Iòÿù¶ó‰7 $docs/api-specification/introspect.md      g‹~¢‚55g‹~¢‚55   ' îU  ¤  è  è   ŞÏÜ5Y N¯Íò`¬éÓ>F +docs/api-specification/oauth_credentials.md       g‹~¢‚55g‹~¢‚55   ' îV  ¤  è  è  IPSB<u°"IÌ.Îh”i "docs/api-specification/password.md        g‹~¢‚55g‹~¢‚55   ' îW  ¤  è  è   Š¾`R“›>£¼ ´¶›Z²s® 'docs/api-specification/refresh_token.md   g‹~¢‚55g‹~¢‚55   ' îX  ¤  è  è  ÊäÜÌs  |s5$OYêæÚ¬—ú $docs/common-oauth2-server-feature.md      g‹~¢‚55g‹~¢‚55   ' îY  ¤  è  è  Ë£Šdî÷.çà‡.ğ¢ÍÈÉxqå 
docs/db.md        g‹~¢‚55g‹~¢‚55   ' îZ  ¤  è  è  mw«~¦¹ç
í]·³ —ı{Ø—¸® docs/deployment.md        g‹~¢‘wug‹~¢‘wu   ' î[  ¤  è  è  )ìP¢†ÃK–‹qÎŒ¼•A3+ docs/design.md    g‹~¢‘wug‹~¢‘wu   ' î\  ¤  è  è   şâğ²ŸfÇëÒ$TùeJOì,7éã docs/devops.md    g‹~¢‘wug‹~¢‘wu   ' î]  ¤  è  è  ÂœáWì5ûğÆè·ûI´Ø docs/env.md       g‹~¢‘wug‹~¢‘wu   ' î^  ¤  è  è   ÷õ<	8ÿ®k´-Q#ÅåÙ docs/flow-diagram.md      g‹~¢‘wug‹~¢‘wu   ' î_  ¤  è  è  6Qæh’İÜn%"²ò•ùğÜ!b§Œ docs/oauth.http   g‹~¢‘wug‹~¢‘wu   ' îa  ¤  è  è  qVóä§óÉi«ÿÆPI¸è¶§ !docs/requirement/user-consents.md g‹~¢‘wug‹~¢‘wu   ' îc  ¤  è  è  e¹XKÏÅƒ¹Ãç1oÊ"¾lQÖ¼— envs/local.env    g‹~¢‘wug‹~¢‘wu   ' îd  ¤  è  è  ah¸kd$—U8÷¯=¦nßùõí envs/production.env       g‹~¢‘wug‹~¢‘wu   ' îe  ¤  è  è  g„ÆóÎ6L.e${x^„f
 envs/stage.env    g‹~¢‘wug‹~¢‘wu   ' îf  ¤  è  è  a\ÍÿòHúçyºIÌ£[ÿ_å envs/test.env     g‹~¢‘wug‹~¢‘wu   ' îj  ¤  è  è  *XfÓ£“Ç×€ı÷gÎoÓó ÑØ ?external/sample_ext_service/domain/sample_ext_service_domain.go   g‹~¢‘wug‹~¢‘wu   ' îl  ¤  è  è  Å¦•l;ôğA!xtŞ¨mZrû Aexternal/sample_ext_service/usecase/sample_ext_service_usecase.go g‹~¢‘wug‹~¢‘wu   ' îm  ¤  è  è  Ü†îX+g×0£?º¢[á½RV go.mod    g‹~¢‘wug‹~¢‘wu   ' în  ¤  è  è  kV¢vp´ÀÍ xÇ?j"ÅïCñGÔ go.sum    g‹~¢‘wug‹~¢‘wu   ' îo  ¤  è  è  ëYtQ¢õS:»…h%5ÅÄÖŠ¼¬^ golangci.yaml     g‹~¢‘wug‹~¢‘wu   ' îs  ¤  è  è  €» ÙÌ”ê¤æ›‘tìe¼ 5internal/article/configurator/article_configurator.go     g‹~¢‘wug‹~¢‘wu   ' îv  ¤  è  è  ÈØõDÏ	°p–Ã¾ò‚ÆŸa 9internal/article/delivery/grpc/article_grpc_controller.go g‹~¢‘wug‹~¢‘wu   ' îx  ¤  è  è  ÕGÔ£8ú‹»ÕZ¸êk„Bç„Ú# 9internal/article/delivery/http/article_http_controller.go g‹~¢‘wug‹~¢‘wu   ' îy  ¤  è  è  ¯(0ƒ,7„æ‡ùSWÉ`ã¬-Mş 5internal/article/delivery/http/article_http_router.go     g‹~¢ ¹µg‹~¢ ¹µ   ' î|  ¤  è  è  û~}È?§a·v&† «®S…PEN 4internal/article/delivery/kafka/consumer/consumer.go      g‹~¢ ¹µg‹~¢ ¹µ   ' î}  ¤  è  è  ïw‚Ú¸ë{«œ‡jéd4äšäUA 2internal/article/delivery/kafka/consumer/worker.go        g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  4‰‹ ¯_jEóË’LÜ'Ràú¼êÏ 4internal/article/delivery/kafka/producer/producer.go      g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  ‘¯ù‚şXî*è‰;\o4qÂ )internal/article/domain/article_domain.go g‹~¢ ¹µg‹~¢ ¹µ   ' îƒ  ¤  è  è  ¢ì½ñö·§W%—\÷Ëé¢Æ· *internal/article/dto/create_article_dto.go        g‹~¢ ¹µg‹~¢ ¹µ   ' î…  ¤  è  è  âœ,°9‹!ıâ²"pôß¾=ù /internal/article/exception/article_exception.go   g‹~¢ ¹µg‹~¢ ¹µ   ' î‡  ¤  è  è  Ô–‰µ4»p¶ ~ÏèÂ"Ñ¿A³ internal/article/job/job.go       g‹~¢ ¹µg‹~¢ ¹µ   ' îˆ  ¤  è  è  ŒÎAƒÌİOĞoÃë»EÈk«¿µ internal/article/job/worker.go    g‹~¢ ¹µg‹~¢ ¹µ   ' îŠ  ¤  è  è  -¹”@„“8MÜŒ'®ò‘÷k2¿' +internal/article/repository/article_repo.go       g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  ‹ü¨dîFÙ'ÂŠâ¸Szt >internal/article/tests/fixtures/article_integration_fixture.go    g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  6Ñ²ªCÓ=€H½QD|ÎNgs :internal/article/tests/integrations/create_article_test.go        g‹~¢ ¹µg‹~¢ ¹µ   ' î‘  ¤  è  è  è²>;f>fì’¦Ï]L}"­ +internal/article/usecase/article_usecase.go       g‹~¢ ¹µg‹~¢ ¹µ   ' î”  ¤  è  è  ’csÍæêh‡iB
Ç!¾ÆœŸ%ü« 9internal/authentication/configurator/auth_configurator.go g‹~¢ ¹µg‹~¢ ¹µ   ' î—  ¤  è  è  ûx!ÖE±KI29y¬¡ÖÃ6A% =internal/authentication/delivery/grpc/auth_grpc_controller.go     g‹~¢ ¹µg‹~¢ ¹µ   ' î™  ¤  è  è  T.2»°° ×L]·½‘bıİZm =internal/authentication/delivery/http/auth_http_controller.go     g‹~¢ ¹µg‹~¢ ¹µ   ' îš  ¤  è  è  …v‡çùãlúÌï€õW|„/ 9internal/authentication/delivery/http/auth_http_router.go g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  ûm)a¨”¿Ïñb!¶ù3KêŞ¸å ;internal/authentication/delivery/kafka/consumer/consumer.go       g‹~¢ ¹µg‹~¢ ¹µ   ' î  ¤  è  è  ï¢T,¿&6Œ<ø6
ka…
pÍwT 9internal/authentication/delivery/kafka/consumer/worker.go g‹~¢ ¹µg‹~¢ ¹µ   ' î   ¤  è  è  4¬Ya–Ò/±)š«UÃRÚ;Û ` ;internal/authentication/delivery/kafka/producer/producer.go       g‹~¢ ¹µg‹~¢ ¹µ   ' î¢  ¤  è  è  %ÈtŸJi“ÖÍK°9h"íı2 -internal/authentication/domain/auth_domain.go     g‹~¢ ¹µg‹~¢ ¹µ   ' î¤  ¤  è  è   ø 'Vò€* ª¥;Xï	nØİ " .internal/authentication/domain/model/client.go    g‹~¢ ¹µg‹~¢ ¹µ   ' î¥  ¤  è  è  W©ïç^ñ°”M¤òĞAæ_?Ò|Ì .internal/authentication/domain/model/common.go    g‹~¢¯ûõg‹~¢¯ûõ   ' î¦  ¤  è  è   ¡ÈóêòK˜à¦Ó“‘±«¤È; ,internal/authentication/domain/model/role.go      g‹~¢¯ûõg‹~¢¯ûõ   ' î§  ¤  è  è  ŒÈğÚº€€êzß^í%O¶û&Í ,internal/authentication/domain/model/user.go      g‹~¢¯ûõg‹~¢¯ûõ   ' î©  ¤  è  è  ÏÅI¾9Nr2 AO.zşğŞ .internal/authentication/dto/change_password.go    g‹~¢¯ûõg‹~¢¯ûõ   ' îª  ¤  è  è  ØİZˆÁ˜ AR4¤ÆRL#Ë .internal/authentication/dto/forgot_password.go    g‹~¢¯ûõg‹~¢¯ûõ   ' î«  ¤  è  è  rÎ›Æ‚“’@|e².ßÔ¯†½) (internal/authentication/dto/jwt_token.go  g‹~¢¯ûõg‹~¢¯ûõ   ' î¬  ¤  è  è  Ê[hEşhƒÓB‡Î¹ÚŸ˜:kMğ³ +internal/authentication/dto/register_dto.go       g‹~¢¯ûõg‹~¢¯ûõ   ' î­  ¤  è  è  Æ–s´U<1ÇÍOòbœæ=:ÛE .internal/authentication/dto/update_username.go    g‹~¢¯ûõg‹~¢¯ûõ   ' î¯  ¤  è  è  CèÜcŞ…
-µï_:?½ 3internal/authentication/exception/auth_exception.go       g‹~¢¯ûõg‹~¢¯ûõ   ' î±  ¤  è  è  Ô¡GJ’+Zpy!ƒË¾	². "internal/authentication/job/job.go        g‹~¢¯ûõg‹~¢¯ûõ   ' î²  ¤  è  è  €a/ 1í^`¡UG O@‰ %internal/authentication/job/worker.go     g‹~¢¯ûõg‹~¢¯ûõ   ' î´  ¤  è  è  KSšt‡ètê¡“¡XïÈÆK²± /internal/authentication/repository/auth_repo.go   g‹~¢¯ûõg‹~¢¯ûõ   ' îµ  ¤  è  è  …ze ò)Ôr,Gë€#dÑHS 1internal/authentication/repository/client_repo.go g‹~¢¯ûõg‹~¢¯ûõ   ' î¶  ¤  è  è  ¢'£*—St¸0ó‡xÚğÓ×¿Sg  *internal/authentication/repository/role.go        g‹~¢¯ûõg‹~¢¯ûõ   ' î·  ¤  è  è  õıbM£8©‡|öãF"McÅ´ +internal/authentication/repository/scope.go       g‹~¢¯ûõg‹~¢¯ûõ   ' î¸  ¤  è  è  •öí«¶'uO-“>p‰ºÓG‹ïKha *internal/authentication/repository/user.go        g‹~¢¯ûõg‹~¢¯ûõ   ' î»  ¤  è  è  ş4 k‹µÛË2·´.ÿø«"&B Binternal/authentication/tests/fixtures/auth_integration_fixture.go        g‹~¢¯ûõg‹~¢¯ûõ   ' î½  ¤  è  è  6Ñ²ªCÓ=€H½QD|ÎNgs >internal/authentication/tests/integrations/create_auth_test.go    g‹~¢¯ûõg‹~¢¯ûõ   ' î¿  ¤  è  è  R*H÷sMU¦¶P´¿·%±&šòL /internal/authentication/usecase/auth_usecase.go   g‹~¢¯ûõg‹~¢¯ûõ   ' îÀ  ¤  è  è  [TìÇûBP|õjÙÿ!C$Ç/ 2internal/authentication/usecase/change_password.go        g‹~¢¯ûõg‹~¢¯ûõ   ' îÁ  ¤  è  è  üà'|¬Iş&ïV·ğİl_Õ+ó 1internal/authentication/usecase/client_usecase.go g‹~¢¯ûõg‹~¢¯ûõ   ' îÂ  ¤  è  è  b,›¤}XÕ°\­£Æ¨ˆY]G0 2internal/authentication/usecase/forgot_password.go        g‹~¢¯ûõg‹~¢¯ûõ   ' îÃ  ¤  è  è  ú5¾œŒ-^èY‘\•âÃÒ¡yÍÎÁ +internal/authentication/usecase/register.go       g‹~¢¯ûõg‹~¢¯ûõ   ' îÄ  ¤  è  è  ä’½=±7{ÁdQÃpEØÊ#˜Pî (internal/authentication/usecase/roles.go  g‹~¢¯ûõg‹~¢¯ûõ   ' îÅ  ¤  è  è  ¥uhFkóLôÃÎGdL” 0internal/authentication/usecase/scope_usecase.go  g‹~¢¯ûõg‹~¢¯ûõ   ' îÆ  ¤  è  è  	s~:È¦YÅµWëŒ<“Ïw†kl˜œ 0internal/authentication/usecase/users_usecase.go  g‹~¢¯ûõg‹~¢¯ûõ   ' îÉ  ¤  è  è  «tÏûºpÄ¬$m@ìvùõoë’ ?internal/health_check/configurator/health_check_configurator.go   g‹~¢¿>5g‹~¢¿>5   ' îÌ  ¤  è  è  õ©4ñp¹O[É0cwOß%1˜sc‰ƒ Cinternal/health_check/delivery/grpc/health_check_grpc_controller.go       g‹~¢¿>5g‹~¢¿>5   ' îÎ  ¤  è  è  |CÆşKbxvE^º£äá½¼—Û=Ã Cinternal/health_check/delivery/http/health_check_http_controller.go       g‹~¢¿>5g‹~¢¿>5   ' îÏ  ¤  è  è  °>µkÂÉaBe­äMÛá=d© ?internal/health_check/delivery/http/health_check_http_router.go   g‹~¢¿>5g‹~¢¿>5   ' îÑ  ¤  è  è  h$&ğ+óóÄwlSÂ£¡9Q0°¼cı 3internal/health_check/domain/health_check_domain.go       g‹~¢¿>5g‹~¢¿>5   ' îÓ  ¤  è  è   çñ£ÿQUî›6ÚõÑØ9h"( ¿ˆ -internal/health_check/dto/health_check_dto.go     g‹~¢¿>5g‹~¢¿>5   ' îÖ  ¤  è  è  {‹ßšø[ş)py‹ĞÇÅg- Hinternal/health_check/tests/fixtures/health_check_integration_fixture.go  g‹~¢¿>5g‹~¢¿>5   ' îØ  ¤  è  è  
&W­)ƒ,IGg¿ÀD a¯cˆ ‚–´ =internal/health_check/tests/integrations/health_check_test.go     g‹~¢¿>5g‹~¢¿>5   ' îÚ  ¤  è  è  ªÊ…ËÇóh^¡•ïB»>G7Ş 5internal/health_check/usecase/health_check_usecase.go     g‹~¢¿>5g‹~¢¿>5   ' îÜ  ¤  è  è  &sËY8¹^
CKñˆÓKÄûhQ Ninternal/health_check/usecase/kafka_health_check/kafka_health_check_usecase.go    g‹~¢¿>5g‹~¢¿>5   ' îŞ  ¤  è  è  ğ NL—×ã»ã¹¨ŠÇËfÜİ•†¤ Tinternal/health_check/usecase/postgres_health_check/postgres_health_check_usecase.go      g‹~¢¿>5g‹~¢¿>5   ' îà  ¤  è  è  d—ì#öy¢@îH¨cğæ	*óHï Rinternal/health_check/usecase/tmp_dir_health_check/tmp_dir_health_check_usecase.go        g‹~¢¿>5g‹~¢¿>5   ' îã  ¤  è  è  P>«²yFIî€ØÅé+S¸³Í@ 1internal/oauth/configurator/oauth_configurator.go g‹~¢¿>5g‹~¢¿>5   ' îæ  ¤  è  è  _æ´‹Æ#şL#…æïĞ†ÁA1›ş 5internal/oauth/delivery/grpc/oauth_grpc_controller.go     g‹~¢¿>5g‹~¢¿>5   ' îè  ¤  è  è  #±€¤ÒKˆU=‘‘ih;£kÈˆCd 5internal/oauth/delivery/http/oauth_http_controller.go     g‹~¢¿>5g‹~¢¿>5   ' îé  ¤  è  è  ÕÙ±¡»(wÄ,q‡Ò¢—LÄ£Yš 1internal/oauth/delivery/http/oauth_http_router.go g‹~¢¿>5g‹~¢¿>5   ' îì  ¤  è  è  óğ‰C“SºØïAç`A²%|1F‰ 2internal/oauth/delivery/kafka/consumer/consumer.go        g‹~¢¿>5g‹~¢¿>5   ' îí  ¤  è  è  çl_,h\Ïq˜;Y/ç Ú 0internal/oauth/delivery/kafka/consumer/worker.go  g‹~¢¿>5g‹~¢¿>5   ' îï  ¤  è  è  ,˜Bİ¹rÜr¤)‹"añ¯“Kàv™ 2internal/oauth/delivery/kafka/producer/producer.go        g‹~¢¿>5g‹~¢¿>5   ' îò  ¤  è  è  DDDâ;ü5ô5I˜rœŸ‰mÁ÷Á +internal/oauth/domain/model/access_token.go       g‹~¢Î€ug‹~¢Î€u   ' îó  ¤  è  è  –xÄ'!;¤-iœ¹I<Iv´‡Ä3 1internal/oauth/domain/model/authorization_code.go g‹~¢Î€ug‹~¢Î€u   ' îô  ¤  è  è   ù@ë>|óÜN°„&k:”ÊªõŒ¸ %internal/oauth/domain/model/client.go     g‹~¢Î€ug‹~¢Î€u   ' îõ  ¤  è  è  Xú»‰LÍlÜÉğ¼ ı2gRï~ %internal/oauth/domain/model/common.go     g‹~¢Î€ug‹~¢Î€u   ' îö  ¤  è  è  EÚ$ÉH
¬sã{@¾(\ä$£Î8¤ ,internal/oauth/domain/model/refresh_token.go      g‹~¢Î€ug‹~¢Î€u   ' î÷  ¤  è  è   ¢™QÍ9±+ç3ŸLS€ZŠ"y)  #internal/oauth/domain/model/role.go       g‹~¢Î€ug‹~¢Î€u   ' îø  ¤  è  è   ºÓõ»t«4Êy°.ğ]ÚÕ1àÅ $internal/oauth/domain/model/scope.go      g‹~¢Î€ug‹~¢Î€u   ' îù  ¤  è  è  ¹k^kùóŸ?W¨şPaú &internal/oauth/domain/model/session.go    g‹~¢Î€ug‹~¢Î€u   ' îú  ¤  è  è  >­p¢z‡vX“’RVvŒÎ•QĞ* $internal/oauth/domain/model/token.go      g‹~¢Î€ug‹~¢Î€u   ' îû  ¤  è  è  /órgxuÉ/û×\9Š,(-4 #internal/oauth/domain/model/user.go       g‹~¢Î€ug‹~¢Î€u   ' îü  ¤  è  è  Ã‚å?…^.÷$K~:ü£Q¦bå %internal/oauth/domain/oauth_domain.go     g‹~¢Î€ug‹~¢Î€u   ' îş  ¤  è  è  	Å0øÙß g–¥»Cİ-u~í &internal/oauth/dto/access_token_dto.go    g‹~¢Î€ug‹~¢Î€u   ' îÿ  ¤  è  è  “Ò§Øe?ÿÒ¨l£\İıøêÔ ëŠ 2internal/oauth/dto/authorization_code_grant_dto.go        g‹~¢Î€ug‹~¢Î€u   ' ï   ¤  è  è  Ûn]·l[JšäòTT;ì~Ä¿ıUL› %internal/oauth/dto/change_password.go     g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  ºîªy¡m¸ƒ™Â–¿ÒÅdÑ 2internal/oauth/dto/client_credentials_grant_dto.go        g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  irFï¼60Àuü!t¤Ë‰–é %internal/oauth/dto/forgot_password.go     g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  ‚~&„„'XÈf“çáT$šâ$ $internal/oauth/dto/introspect_dto.go      g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  ’xE•Aí~ÏRa5B™T=A internal/oauth/dto/jwt_token.go   g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è   XÜ`ò³¶Ô»xvRk ¯O"ĞÄ
 internal/oauth/dto/oauth_dto.go   g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  Î—£J×‚gµ9Ò1J9K„lgİ (internal/oauth/dto/password_grant_dto.go  g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  
_Ï kUNÛ­UÊÔâŞ€²5 'internal/oauth/dto/refresh_token_dto.go   g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  Şdù¹»©~=ŠºúÃÜ·=“ ïCÔ "internal/oauth/dto/register_dto.go        g‹~¢Î€ug‹~¢Î€u   ' ï	  ¤  è  è  ğè‰³ÔÆHØn8Dùj@å¶€ %internal/oauth/dto/update_username.go     g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  	“hŸ`1§¯à J$!©_ +internal/oauth/exception/oauth_exception.go       g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  ÌdÑ1
±ÿñW•Ç³('é\Wß}Œ internal/oauth/job/job.go g‹~¢Î€ug‹~¢Î€u   ' ï  ¤  è  è  ªŒxbÀ×\Î‰n´0Ö¤	Á@ internal/oauth/job/worker.go      g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  	ô3Â­ÇgËü@®Ü
ñë»èÊn )internal/oauth/repository/access_token.go g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  KÀlØ¾#D7+QWëàôpÈ²ğï )internal/oauth/repository/authenticate.go g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  ;®=¸Ï#°ËxŒ-&˜±‡ /internal/oauth/repository/authorization_code.go   g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  „<‘ZL½a)øt$?ñ¥¨pâ^ (internal/oauth/repository/client_repo.go  g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  ³aÆÒ¡³	ÈÑchÀÜ -äb¹‡0 :internal/oauth/repository/grant_type_authorization_code.go        g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  ÖÜ>,c'Gu4ÙğuŒ¶SKşœ 'internal/oauth/repository/introspect.go   g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  CX4lî2°(í{Ÿpx:b>X 'internal/oauth/repository/oauth_repo.go   g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  Ã*”XBİ´àË¯A<BRÓÊÿ *internal/oauth/repository/refresh_token.go        g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  šoI»çôÁ>†ZßNŞ7X !internal/oauth/repository/role.go g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  öz/+8Õ@3D½|ë
¬œ¢ãA; "internal/oauth/repository/scope.go        g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  €Ÿ,z7¢ÏtêÏ€tÏ©Á ¢0ü !internal/oauth/repository/user.go g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  ÑõÍ+4Ì¸}«Ğ;æKÂúzA  :internal/oauth/tests/fixtures/oauth_integration_fixture.go        g‹~¢İÂµg‹~¢İÂµ   ' ï  ¤  è  è  6Ñ²ªCÓ=€H½QD|ÎNgs 6internal/oauth/tests/integrations/create_oauth_test.go    g‹~¢İÂµg‹~¢İÂµ   ' ï!  ¤  è  è  \oıªİàiJô4¾,O~çE )internal/oauth/usecase/change_password.go g‹~¢İÂµg‹~¢İÂµ   ' ï"  ¤  è  è  öÄE­|³
.ÍÜé”Ì‰@ˆ–Q (internal/oauth/usecase/client_usecase.go  g‹~¢İÂµg‹~¢İÂµ   ' ï#  ¤  è  è  cT:ì–¼Ïp±ûE4ŠP8Fu¤ )internal/oauth/usecase/forgot_password.go g‹~¢İÂµg‹~¢İÂµ   ' ï$  ¤  è  è  6cKl¬:­ÜÇï
Fİİ ,internal/oauth/usecase/grant_access_token.go      g‹~¢İÂµg‹~¢İÂµ   ' ï%  ¤  è  è  †Vßu‰z {á‚b CU©Ê 7internal/oauth/usecase/grant_type_authorization_code.go   g‹~¢İÂµg‹~¢İÂµ   ' ï&  ¤  è  è  š0¦HÎÆ{˜œ¤'ÃÕi¹àXœàş 7internal/oauth/usecase/grant_type_client_credentials.go   g‹~¢İÂµg‹~¢İÂµ   ' ï'  ¤  è  è  ÑÏ³Æ×ä0[²e1¡ÄÎ%•­Z -internal/oauth/usecase/grant_type_password.go     g‹~¢İÂµg‹~¢İÂµ   ' ï(  ¤  è  è  –›šö—qf–ˆõJï¾ŸI[¸0£ú 2internal/oauth/usecase/grant_type_refresh_token.go        g‹~¢İÂµg‹~¢İÂµ   ' ï)  ¤  è  è  «åø×æ”Vl@{À‹TtUï–@' $internal/oauth/usecase/introspect.go      g‹~¢İÂµg‹~¢İÂµ   ' ï*  ¤  è  è  nˆÖÑjËˆ\Å£›»nJ†²pëÅ internal/oauth/usecase/login.go   g‹~¢íôg‹~¢íô   ' ï+  ¤  è  è  P?i Ù×+ßğGKµšÁ^Ü€lä? 'internal/oauth/usecase/oauth_usecase.go   g‹~¢íôg‹~¢íô   ' ï,  ¤  è  è  Î>„By\w'sjÍ˜5'È½C‘ 'internal/oauth/usecase/refresh_token.go   g‹~¢íôg‹~¢íô   ' ï-  ¤  è  è  Ô¥U¦W»CÁO¥_ÏV×g¡œ "internal/oauth/usecase/register.go        g‹~¢íôg‹~¢íô   ' ï.  ¤  è  è  Ié{üú§:Äïbörãƒ™{ÿY7 internal/oauth/usecase/roles.go   g‹~¢íôg‹~¢íô   ' ï/  ¤  è  è  ¦;Ñ„Ğ„?¹ˆ%:›I1L¢  'internal/oauth/usecase/scope_usecase.go   g‹~¢íôg‹~¢íô   ' ï0  ¤  è  è  ;‚´:ÚJaÃœÓšòPêiP¿
 !internal/oauth/usecase/session.go g‹~¢íôg‹~¢íô   ' ï1  ¤  è  è  	kb×¿ñ‰Ê'wpd†ƒ3ÃÀíÙå 'internal/oauth/usecase/users_usecase.go   g‹~¢íôg‹~¢íô   ' ï2  ¤  è  è   eÅW’ñS{Es¯¤—¹“òr¶w™ main.go   g‹~¢íôg‹~¢íô   ' ï4  ¤  è  è   %ï;´n4ÖBĞ‰:–y±Ï™Ò) pkg/constant.go   g‹~¢íôg‹~¢íô   ' ï6  í  è  è  #N4j?` 9i €WâÙÅ[?Äø pkg/constant/constant-temp.go     g‹~¢íôg‹~¢íô   ' ï7  ¤  è  è  NÔ¬E•A¢IË²ÈçVkÀÒ¸ş%—Ä pkg/constant/constant.go  g‹~¢íôg‹~¢íô   ' ï:  ¤  è  è  ‘6ùEy`bíÁ	LÁïİgdÈ]3PP +pkg/constant/error/error_list/error_list.go       g‹~¢íôg‹~¢íô   ' ï;  ¤  è  è  7pç;¿İ4X‚É­ŸGÕ¢íú)y !pkg/constant/error/error_title.go g‹~¢íôg‹~¢íô   ' ï<  í  è  è  Ü?ÈÍÓ›$ùˆ‚Va±›–ãÊ- pkg/constant/httputil.go  g‹~¢íôg‹~¢íô   ' ï>  ¤  è  è  ëwpæ×­úBRÇÂ›‰`
u?×>M pkg/constant/logger/logger.go     g‹~¢íôg‹~¢íô   ' ïA  ¤  è  è   {HŞ¡Úaœ¶–ıb™À:ÖÙ7v—  pkg/error/contracts/contracts.go  g‹~¢íôg‹~¢íô   ' ïC  ¤  è  è  ed};²éhšå;î¹“Úâ	ÿ +pkg/error/custom_error/application_error.go       g‹~¢íôg‹~¢íô   ' ïD  ¤  è  è  V,Ì· >5ñ&F q­³şLf n +pkg/error/custom_error/bad_request_error.go       g‹~¢íôg‹~¢íô   ' ïE  ¤  è  è  8?6„Â¬“ë½ÜúÕd"¼§ªP`Ö (pkg/error/custom_error/conflict_error.go  g‹~¢íôg‹~¢íô   ' ïF  ¤  è  è  Éé$c¹!ˆ˜„¯ºÎuàV"•‰% &pkg/error/custom_error/custom_error.go    g‹~¢íôg‹~¢íô   ' ïG  ¤  è  è  Ö¡¿¢Utİ`®İt¥÷L2²T &pkg/error/custom_error/domain_error.go    g‹~¢íôg‹~¢íô   ' ïH  ¤  è  è  Gş|ÈÑzÄÅšÁŸTh( Æ=B (pkg/error/custom_error/forbiden_error.go  g‹~¢íôg‹~¢íô   ' ïI  ¤  è  è  €:Æ1Nfô:o•ı +şJÜC±‡ /pkg/error/custom_error/internal_server_error.go   g‹~¢íôg‹~¢íô   ' ïJ  ¤  è  è  V5“Jf³“|bÇÂş 	kÔ0i‡! *pkg/error/custom_error/marshaling_error.go        g‹~¢íôg‹~¢íô   ' ïK  ¤  è  è  8¬X74iûQxÉF€SÏXš«X )pkg/error/custom_error/not_found_error.go g‹~¢íôg‹~¢íô   ' ïL  ¤  è  è  tÉ¸k£˜=zQ:¤ÒÄ–áŞ¬W ,pkg/error/custom_error/unauthorized_error.go      g‹~¢íôg‹~¢íô   ' ïM  ¤  è  è  tİ›£DHZ.o¦Øë…%Ñ%ëS# ,pkg/error/custom_error/unmarshaling_error.go      g‹~¢íôg‹~¢íô   ' ïN  ¤  è  è  îÍsë‡ lv_µôÁ9I\RÿÏ *pkg/error/custom_error/validation_error.go        g‹~¢íôg‹~¢íô   ' ïP  ¤  è  è   §ÌE4ÕûWhgK¤?Ò%]ºà  $pkg/error/error_utils/error_utils.go      g‹~¢üG4g‹~¢üG4   ' ïR  ¤  è  è  ôåX½’Ä
µ‡%9‘7í
® #pkg/error/grpc/custom_grpc_error.go       g‹~¢üG4g‹~¢üG4   ' ïS  ¤  è  è  ÷ºËû†~Å5Å´à\{’û pkg/error/grpc/grpc_error.go      g‹~¢üG4g‹~¢üG4   ' ïT  ¤  è  è  
#Ñ±#cögp³5©“¿Æì°ö9 #pkg/error/grpc/grpc_error_parser.go       g‹~¢üG4g‹~¢üG4   ' ïV  ¤  è  è  ¾êæçHìæ­×¼Ú[dQÄ…²É×Ç #pkg/error/http/custom_http_error.go       g‹~¢üG4g‹~¢üG4   ' ïW  ¤  è  è  |S1ä
üws63áLBë‰ ‡Ğ pkg/error/http/http_error.go      g‹~¢üG4g‹~¢üG4   ' ïX  ¤  è  è  i‰2*BZ÷i–Ä–dºG #pkg/error/http/http_error_parser.go       g‹~¢üG4g‹~¢üG4   ' ïY  ¤  è  è  Ãe |f>±ô#&b1Â"¨(¹—) pkg/password.go   g‹~¢üG4g‹~¢üG4   ' ï[  í  è  è  tÒ¾°¹³•8ÎŠ~3%vviÊy pkg/response/error.go     g‹~¢üG4g‹~¢üG4   ' ï\  í  è  è  9Üß@>«äú¡ë]g9Ü¸˜`†*“÷• pkg/response/response.go  g‹~¢üG4g‹~¢üG4   ' ï]  ¤  è  è  *²Fµ‘9¿p¬çÌP|¬ìøÆ® 
pkg/sql.go        g‹~¢üG4g‹~¢üG4   ' ï^  ¤  è  è  ğUiQuK©¬Ü_F~‡g+ÊD pkg/string.go     g‹~¢üG4g‹~¢üG4   ' ï_  ¤  è  è   ÁÊÿë¾³C,½k:;ÏY‘ä pkg/utils.go      g‹~¢üG4g‹~¢üG4   ' ïa  à   è  è    …xË¡{uO(1jx¢‰Â¥ßcs3] web/landing       TREE  c 219 11
i´‘ŒvÍ±Äjú	áFƒ	¢©db 21 2
C"ƒÒˆ^
×‰æKhŞô'6*Îfixtures 5 0
îiqé’^(s9\`ãWÚmigrations 16 0
¸[P‡Õ<k«ÿ ÇB:k·×¿mapp 1 0
ƒ?·à±Ö¿x_|ƒcıXoX™cmd 3 0
¶YÂ`VYdŞşï·5‡	°ïpkg 33 3
8ääQeO¡÷0¸dëôgerror 20 5
7ãæº$jÜ ½E>Óö4·ËÕgrpc 3 0
XyF¾°îªœùlÁPYY|©•http 3 0
Øó—|`{Wè$KÏ``tQ D÷ÿcontracts 1 0
¢Çóål<èäy†° ¹$‚a½°³error_utils 1 0
×:“ÑØioõ*÷ş×à¸[Çòcustom_error 12 0
Ş»:b¦Gx‘#ó FÈ>×constant 6 2
›(t·55Ê€-d ³ÛÃTHÉ*Œerror 2 1
³¥åS'ÿ©Æü2uıèØ•¾error_list 1 0
ÿŠ÷>‡ÊŠõEŠâi áfJòlogger 1 0
d1ó¼ú7Tìâ-àJgDXíªresponse 2 0
¾ƒ©Y#İ¯¡«V÷0ë¨~y¥(¸web 1 0
bwåëÖš0®gc´Ë‘tgBĞšdocs 15 2
ö8`·?ºé­ƒÔäÆòî6¢56requirement 1 0
0‡}X0HslbóşLùÔ6§wÆ|api-specification 5 0
ÁsRPÌ£ˆÚ`ÕBkm¿ÙÂenvs 4 0
g§¥ônßxOn‚VÀ›õ®¼"config 1 0
dù‘©‰eñĞÒ¤q–nO…fÁexternal 2 1
›å ’:>A$7µ%[^#ÈCsample_ext_service 2 2
Ô‘l=zÛ~§*$eÒ«€¦xÃÏ=domain 1 0
ß‚–óÇ÷€HÆ¨®9üyusecase 1 0
¯Dà7WˆÑØmª!üAÚ“UoFinternal 126 4
–àêı<îYrmD3LL0"çoauth 63 9
ÈËÔœÊQ£„r„ïfb^ ƒÙêÕdto 12 0
¢Ñıçj…|0¹Áá@Ãÿóÿ{wÏ£job 2 0
7v™³ëÒÃ°à0ãvSØ3Gn/qtests 2 2
÷å­Á{;rrª3yø&‘UÇûNfixtures 1 0
ôº5ac±×‹€Ùw*Š#˜€£integrations 1 0
€qq‡)øûÖ`‡ï»}÷/¦domain 11 1
?ûe©9ş¹Ø‰>ÆdH5model 10 0
¤¡,2Š^¢]®nò/x ê‹usecase 17 0
a£¦ˆä6-™³ñ3G
ÈyLrdelivery 6 3
ƒD’¼‘Ÿ¦ÌÙ^å¡2—„n¹Ñgrpc 1 0
YbRèóìÂã³—Ã5$dß%<‚9http 2 0
YîÆÙyíş¦¹µÌÂÿlRU³- Kkafka 3 2
òò£8CËº`İÁ]²@¬+ä#­consumer 2 0
K[sñ¹ÜhßmÕ¦ì4×­|U%Ô¬producer 1 0
¬‹hÛkf×A‹@“dWºïexception 1 0
p¬õj2éL2MäÆu2Iô;repository 11 0
¢&J$s÷@§-mcPœ˜¯@configurator 1 0
Fú† ¯_”èÛ¾²SËÚ²œÂarticle 16 9
ˆu”ÏQó&9ºÛaöSõ%‚¿dto 1 0
¸æÂ¯Ìú³Ìy˜]¾ƒ¡ˆoV¸ájob 2 0
´ƒwX‡7F•Ïeé oÒ>qùtests 2 2
¦:ÊãN;äJf9#Mïn£ıfixtures 1 0
»›Ìi‘–uXö€Gµ>;ùéNintegrations 1 0
,ûšñuEàUs0’Œ6ÆÎ¾y][domain 1 0
ÁÔ!œ<O!í¬}s 	Îû§usecase 1 0
oÁK+,Û80­D‡nãê;†è®”delivery 6 3
Øö2Åùišè…&VKÛ*Ûoğ^.grpc 1 0
ª	!wî€8Ò³;)¥_çUİhttp 2 0
`iqÎà;Á´V[ó	Ú5İk%±±kafka 3 2
Ê)(üÉ}{*Î(½S©“#n´consumer 2 0
}€DXìoê2Ê$9LÃk¤PÕ+Èproducer 1 0
KÉÅ™€†Ï
L£{2¥©ãzexception 1 0
!a_Bƒ¤ªd}#³ÑÊÔ<÷repository 1 0
í-ÖXËï°ã –ş ôƒè[xõconfigurator 1 0
<‹‚óW­qL„Ò1euqdhealth_check 12 6
ˆó]K¿‹ŸôMùƒ‘ÿj:WwûTldto 1 0
@·²¼Q†YVºô%»,Ê 	nv²tests 2 2
VHxAÌn"•‘Öc¸‘İ^\qLfixtures 1 0
Æ>ÌØ†·?õ½9[h4^¬!ô—integrations 1 0
Ñÿûà¢·¤#É§Å	Y¢£S›domain 1 0
)ÙÆtFR¡ƒnÒå'è¾ŒúLusecase 4 3
WI6×=k®îuó¹óÚÅ»ÿÊkafka_health_check 1 0
*Fê#œC°‹¦ş'ç6Ûöitmp_dir_health_check 1 0
~îrÙøJË`Gƒp*²ê‰‡postgres_health_check 1 0
èg`1:q
Sú+xÌ é	¤8Ûdelivery 3 2
tJ};ùß-ììğ)hqgrpc 1 0
GĞ	06Ù2=ƒè¡/Aå[T‡http 2 0
VßS‘Jt›oü(=£‘÷configurator 1 0
ÜÂp÷ìK²âŠfN»†ş†	à°authentication 35 9
F›+ËL¿d¼#÷‰§:­äíµEdto 5 0
E´ÿÈüÖÇW‹Ñ}IJ`¿Mœjob 2 0
âAçÁvgMÒ/•PwDò	ètests 2 2
=?c?ô’­Ş‚Õ-S¶öNJ–?fixtures 1 0
ÊàÏı!§6¬fºÊ †k(`x'Cintegrations 1 0
ÃĞÖR²Ø‡‘ˆ}<jÑLZÑ+^domain 5 1
ø‚«¯z<{ws– ÊŸL‰model 4 0
Uê9²h÷ÁkVæøî¦oÆíî9usecase 8 0
éşo¾ZDÒÚ¹P¯§Ôˆƒì‚Hdelivery 6 3
šF‚³iÜz/˜~r@m)=ß¢grpc 1 0
3|
ÎÈ YW’TÑ¦~kÍ http 2 0
¶v÷¶Ğ¬0¹ÉQ
®ãMÊ=–kafka 3 2
 ‘p3WyLÛ tJ‘Ş€Ô¼|Ïconsumer 2 0
­Æ“{ŠÒÜÏ	\¦§8’ßproducer 1 0
¦¹3!N¹×gv àKËMtK}-ı=exception 1 0
#zcÂûôÕCØŒ±Àa:¸²ˆ<repository 5 0
ÓÄvôeËú\5÷šlnMconfigurator 1 0
ĞÕtÁÃê‘ ‡âcj¾£˜€ymdeployments 3 0
PëÛ€·!ÕÄâÑò)?Êİ	OÄò~][lcvšså4&î7;
´Åó

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
ÿtOc                  	   
                                 "   (   *   .   0   3   4   7   8   ;   <   =   ?   @   B   I   I   J   J   L   M   O   O   Q   U   W   Z   [   ^   `   b   e   i   i   j   l   m   p   q   s   v   x   {   €   ƒ   †   Œ      ’   ”   •   ™   ›          ¤   ¥   §   §   ª   ¬   ­   ±   ²   µ   ·   ·   ¸   ¼   ¾   À   Ã   È   Ì   Í   Ğ   Ğ   Ñ   Ò   Ò   Õ   Ù   Û   à   å   æ   ç   ê   ë   í   î   î   ï   ğ   ñ   ô   ö   ÷   ù   û   û   ı           
                        !  "  &  '  )  +  .  /  3  :  ;  <  >  ?  @  @  C  E  H  H  J  O  P  S  U  W  Y  [  a  b  e  f  g  i  j  m  p  p  q  r  s  u  u  w  z  }  ~  ~  €  €  ‚  „  ‰  Š  ‹  ‹        –  —    ¡  ¥  ¨  ª  ®  °  ´  µ  ·  ·  º  ½  ¿  Å  Ê  Í  Ğ  Ğ  Ó  ×  Ù  İ  à  á  ä  å  è  ë  í  î  ñ  ñ  õ  ö  ú  ú  û  ı  ÿ            
                       #  $  &  '  ) 'Vò€* ª¥;Xï	nØİ "€y÷ÿó=¥„Eexó)[tPl_,h\Ïq˜;Y/ç ÚudM¬œ69Ş}Õğ%C±sÿE´ÿÈüÖÇW‹Ñ}IJ`¿MœÈëˆ•”–ĞÒ°¶˜¼rs.†ë»@ø0­"XyØÑë‘ä[ùWí©÷éÀC—¿½S9ÃŸ/–»P¾˜â&6ÿŠ÷>‡ÊŠõEŠâi áfJòÉøfS˜Õvé5ÑÓÍ=à7‰dù¹»©~=ŠºúÃÜ·=“ ïCÔ¹k^kùóŸ?W¨şPaúº„ĞhJQŞ^éÛû9QlôÈè¯v‡çùãlúÌï€õW|„/ç¨çd'JGĞá”"Ùƒ*´ùÂ,©gVK2JvÙâuÁ„srüP¢t„‹p„›ÏÀ:tJ};ùß-ììğ)hq°jŞÙT¥4[ÎTVñ@NˆÈ…›Ëõ÷ú9İ¢Ÿ¯}4}õYŒı2Ğ¬BÏD²0~aNl–¬ªŒxbÀ×\Î‰n´0Ö¤	Á@L¢sS~ğ»G
Kgı¼I¼èıbEf ¦º<JÏ	…-èLºXVßS‘Jt›oü(=£‘÷£±JÔq(Ÿ’,rÕ`Fœ^ÙI$ø5>Zqã±Ç#\MÊ‰n-g“Ü-÷º*\½»^õÍrÌ,¦•l;ôğA!xtŞ¨mZrûéß¢OµV˜{ º”ZázPS«ön0æ£XW°Ü¼ •ÀY ò}i*5~¶Qz{ƒWn65ó’ë‚í~DVÓTe~¡í¶¾Õ®ÅZá~?û«Ô_l?FÛB÷Ï"e!
’]í¶§‚.õöœÑ4lVßu‰z {á‚b CU©Ê\ÍÿòHúçyºIÌ£[ÿ_å·­qÕ{ÂN’]N ×IƒxÈóêòK˜à¦Ó“‘±«¤È;÷ºËû†~Å5Å´à\{’û'c;€â¶ØAlæãÆí´“n[j™QÍ9±+ç3ŸLS€ZŠ"y) F›+ËL¿d¼#÷‰§:­äíµE•,×\]ÌRšc°Pğ/›Å¿ˆŸ,z7¢ÏtêÏ€tÏ©Á ¢0ü²¹#l+†«“û7àqÚ†Y/µç?ŒåWŒ€©`Ø'_2‹’´¸2n*•¿Oß·]­ñh«sKÉÅ™€†Ï
L£{2¥©ãz€qq‡)øûÖ`‡ï»}÷/¦ğQÆ_A—ØÂîîágµ«µã¸[P‡Õ<k«ÿ ÇB:k·×¿mšŒ…if–”K£‚‘å`5Ñ¬X74iûQxÉF€SÏXš«XÅ0øÙß g–¥»Cİ-u~íh.ìÊT¼ãÄÖï² d»ßk) NL—×ã»ã¹¨ŠÇËfÜİ•†¤?ûe©9ş¹Ø‰>ÆdH5tÏûºpÄ¬$m@ìvùõoë’¬Ya–Ò/±)š«UÃRÚ;Û `åDßiõ’†ˆU•?AuK”³jbwåëÖš0®gc´Ë‘tgBĞšášrFHmà4­uÜï^}¿Î$&ÒP´¢Ğjs­/ÍÔ¶àÕz/+8Õ@3D½|ë
¬œ¢ãA;òP,Á«õbòò»lÇ|»İT¢õ–†ƒ…™®œí1ûoª“Sšt‡ètê¡“¡XïÈÆK²±i´‘ŒvÍ±Äjú	áFƒ	¢©Âä©ÑËb+İ¡VÖ/‘§&Ç Ísë‡ lv_µôÁ9I\RÿÏÜ`ò³¶Ô»xvRk ¯O"ĞÄ
İZˆÁ˜ AR4¤ÆRL#Ë!a_Bƒ¤ªd}#³ÑÊÔ<÷#zcÂûôÕCØŒ±Àa:¸²ˆ<#Ñ±#cögp³5©“¿Æì°ö9$&ğ+óóÄwlSÂ£¡9Q0°¼cı%4ZOãR¹X©¨”!]t¼F%*™$•ãa—B- ©ûpš_fÌ?Í'£*—St¸0ó‡xÚğÓ×¿Sg '»ÚoW…Ö†š“0l°²h-ğ(0ƒ,7„æ‡ùSWÉ`ã¬-Mş(B«óÖ×úm­	Å?&¾‚/W(GåÆ	ı×’4œ k£æ-ˆŞ(ÿö~.©cfµ3¯T®±fŞ)-LeWÃ€H„jqˆ7à<)ÙÆtFR¡ƒnÒå'è¾ŒúL*Fê#œC°‹¦ş'ç6Ûöi*H÷sMU¦¶P´¿·%±&šòL*”XBİ´àË¯A<BRÓÊÿ+jÔ÷tj0	r6p`zÄVcmŠ,Ì· >5ñ&F q­³şLf n,›¤}XÕ°\­£Æ¨ˆY]G0,ûšñuEàUs0’Œ6ÆÎ¾y][-Â›¸8”’eÊ¸M>rà&íHñ-÷]í²O€¾PR0ÂÄXx.‰ø«yYşsÌşEÑ”&…¸'¬:#.2»°° ×L]·½‘bıİZm/órgxuÉ/û×\9Š,(-4/ĞéŸ6[âYUôß{r	±Ñš¾x /ó?ÁúÙ}•İ´#MĞ~ïNDÙ0=Énö³¦Wœ£qy~0‡}X0HslbóşLùÔ6§wÆ|0¦HÎÆ{˜œ¤'ÃÕi¹àXœàş0³«Â¡(æ«ŸÒë=eìcsú2ĞoyĞ&j »oş³U±ó Íà3|
ÎÈ YW’TÑ¦~kÍ 3¡O!E„¦ˆW`¸ß8ú¤µh”4ºT%!¯ÊuL†Â@6¦B5pSÇ Xªï	œ{ üw+NŞ5“Jf³“|bÇÂş 	kÔ0i‡!5¾œŒ-^èY‘\•âÃÒ¡yÍÎÁ6ùEy`bíÁ	LÁïİgdÈ]3PP7ÄÕÀ:{6®ešt!ºÍ|Å¦7ãæº$jÜ ½E>Óö4·ËÕ8aWe[ãûÌĞÏ†ZÈ.A°Í8Š.5_r12²	_†È§568ääQeO¡÷0¸dëôg9Èµ^	âb*º“®Âtw’8z9ÛGñkpülÈá1Ùí˜¬Ë+â:dYª‘_şÜ¢H¦±Ë+:Æ1Nfô:o•ı +şJÜC±‡:ûÌƒÑûÏeÙhT¢ÅâÑLŞ%;J/ÙHJañÆf~Ì¿ûF;1¥h´îÒŸEug4ñA5;Ô;ÂıLu÷ªXÇ™»ùÉîq¦şE:p;Ñ„Ğ„?¹ˆ%:›I1L¢ ;ÒëÊø‘¹‚±¼WÍøGc<‘ZL½a)øt$?ñ¥¨pâ^<?F¢Ën)E¨%òÍŸ]­Z¼ü<‹‚óW­qL„Ò1euqd=?c?ô’­Ş‚Õ-S¶öNJ–?=r5³ö5BŸÊ¯(&†ÿÎÄ"={˜™c¥MôAé7-ZE¶6\>ÿIx\h0!İÕIhO¡Ãæ>„By\w'sjÍ˜5'È½C‘>«²yFIî€ØÅé+S¸³Í@>®“Dºaº76NyC )ÅFä>µkÂÉaBe­äMÛá=d©>ç³«’IÃş‘µâF¥f— ?6„Â¬“ë½ÜúÕd"¼§ªP`Ö?i Ù×+ßğGKµšÁ^Ü€lä??½ïUœdMù!ŒGùÅù›\6â¿p?ÈÍÓ›$ùˆ‚Va±›–ãÊ-@·²¼Q†YVºô%»,Ê 	nv²@ë>|óÜN°„&k:”ÊªõŒ¸A£ê3?2ç\ùéS*Ò)„=A¥ï™|¾¯_İ¶ı5càµœ5Í%_BÙuC)ãñÁ±À&	²g¾aíàˆC"ƒÒˆ^
×‰æKhŞô'6*ÎCMgÌòHÅ§tµz;¦øMĞE§uCÆşKbxvE^º£äá½¼—Û=ÃCèÜcŞ…
-µï_:?½DDâ;ü5ô5I˜rœŸ‰mÁ÷ÁDzŞVæ®Cİßs3 ÆqW‰3ºEØ°¶XW"4Ë=iò Eâ6Ó_k?Põ(m/…Q3ƒèæFÅü¤Ô'Å±â††AÜ¶è-A|F×=j¤ìß”7^[úÂ.Fú† ¯_”èÛ¾²SËÚ²œÂGw>,%§Ü¦ÔD+9å¹Ğ†rG²êøEEË
`i*%‰ğâŒ=…GĞ	06Ù2=ƒè¡/Aå[T‡GÔ£8ú‹»ÕZ¸êk„Bç„Ú#HŞ¡Úaœ¶–ıb™À:ÖÙ7v—IÛ| rP: ©––˜âİÃ~“ò¨Ié{üú§:Äïbörãƒ™{ÿY7K[sñ¹ÜhßmÕ¦ì4×­|U%Ô¬KÀlØ¾#D7+QWëàôpÈ²ğïKÚ*]•²šPITRt]`åcLX“m“U²Ò¹ò—²'mL]#x®¶Iì¢Š•¨ê8]`M75é¿æÍø°«kâüÒ1€„>NŞ3jÂ¶K"$LÑêMƒï¾–N4j?` 9i €WâÙÅ[?ÄøNj ‘İ5DØòSôT
Ù=BêN‹kÉ=Êû¼¿«ì;¼€yOLÃwqÂ.Hğ¢ÕóÎ’<³PSB<u°"IÌ.Îh”iP¢†ÃK–‹qÎŒ¼•A3+PëÛ€·!ÕÄâÑò)?Êİ	OÄQ><‘¿h/ª<İGKq+*A±Qæh’İÜn%"²ò•ùğÜ!b§ŒS1ä
üws63áLBë‰ ‡ĞT:ì–¼Ïp±ûE4ŠP8Fu¤T<‹–$a€ëw²í×è8sˆrTÃ_v¾@¿’&]—ø”¬Øô¤¾™?TìÇûBP|õjÙÿ!C$Ç/UiQuK©¬Ü_F~‡g+ÊDUê9²h÷ÁkVæøî¦oÆíî9VHxAÌn"•‘Öc¸‘İ^\qLVóä§óÉi«ÿÆPI¸è¶§WI6×=k®îuó¹óÚÅ»ÿÊW­)ƒ,IGg¿ÀD a¯cˆ ‚–´W®p¬÷ø^?HA`.Õõå)ø¸²ÍX4lî2°(í{Ÿpx:b>XXNY§IôİÔm¢Œ¬8œTÓXfÓ£“Ç×€ı÷gÎoÓó ÑØXyF¾°îªœùlÁPYY|©•Xô¸û††:„5ÚµL>a6•ËYbRèóìÂã³—Ã5$dß%<‚9YtQ¢õS:»…h%5ÅÄÖŠ¼¬^Y‘ê~¸š…]«§‚õi«aSÍÔYîÆÙyíş¦¹µÌÂÿlRU³- KZOÎMT“WJ`zØ¦qşÓáFK“[Q&ïÅ™³-íĞ¼g®ıÀÅl"[hEşhƒÓB‡Î¹ÚŸ˜:kMğ³[••®åUàÛp[	.Ş™´j±%#]Ó›CRµÆ.3<Äg½¹øü”ö^+UEáÊ¾ØæÙı(?O¢g©Â`iqÎà;Á´V[ó	Ú5İk%±±`v¡]JEâ=^¯qHæ¯’`ÕªC_ÎÄd"~Eğ}îÊ„pòa£¦ˆä6-™³ñ3G
ÈyLra-ÕŠ†ëOIë›å§—\iaÆÒ¡³	ÈÑchÀÜ -äb¹‡0aáà?‰QB½`´õ· hCË*yb´'¸a9MÚñ™Ì‘-J3¼Tb×¿ñ‰Ê'wpd†ƒ3ÃÀíÙåcKl¬:­ÜÇï
FİİcmDœ<Úí	ÓøD‡¸òKcsÍæêh‡iB
Ç!¾ÆœŸ%ü«cæŞB'µRLßÿf,0
‰-ù4c÷/î=&z±mµ¿E~İd !$Ä8EBCFı ¬Q@bæ¡b
d};²éhšå;î¹“Úâ	ÿd1ó¼ú7Tìâ-àJgDXíªdÑ1
±ÿñW•Ç³('é\Wß}Œdù‘©‰eñĞÒ¤q–nO…fÁe’ÿîÇ¶›İ^\Ÿæg{Ì#°ÀfDôOä?a‡÷QwÖ’Šlå@§g§¥ônßxOn‚VÀ›õ®¼"gA?ğÇÔ•šµŞØf|úMªgÀ˜V§¤U¤§ªşRœ–§	h¸kd$—U8÷¯=¦nßùõíirFï¼60Àuü!t¤Ë‰–éi‰2*BZ÷i–Ä–dºGj”J	U°Åš=¬ù÷c!ı÷lé…4&_N)mÿ`}o ¬Go®m)a¨”¿Ïñb!¶ù3KêŞ¸ån]·l[JšäòTT;ì~Ä¿ıUL›oI»çôÁ>†ZßNŞ7XoÁK+,Û80­D‡nãê;†è®”oıªİàiJô4¾,O~çEp¬õj2éL2MäÆu2Iô;pç;¿İ4X‚É­ŸGÕ¢íú)yqÉ/–c€€Â(BnZ ;ñÛ‚r853ÔDÛĞ8…i2+D&Q²nrÎ›Æ‚“’@|e².ßÔ¯†½)sËY8¹^
CKñˆÓKÄûhQsló4«§äÿ(NºQXu*äÚ“uhFkóLôÃÎGdL”uÇÙå‹÷·¾šƒ¯¬VO€wh«v/öÜse$6¶€Ô££tæS«vf´VşÅÀ¿æ‡8oÅ„ÎM‘v®‹ ùéšhóÚ7ôşû­€tÒ.w*¾îÃ,vzSÚs/-q)Şwpæ×­úBRÇÂ›‰`
u?×>Mw‚Ú¸ë{«œ‡jéd4äšäUAw«~¦¹ç
í]·³ —ı{Ø—¸®xl’ÀOFËµ˜zçl«xÄ'!;¤-iœ¹I<Iv´‡Ä3yÚf|$CÎ“ûv$ºgÑCÍ¢Díz[Í¦r?WÆú®\Z9¥üt,êÿze ò)Ôr,Gë€#dÑHSzr™­I|Zo7-9ıwb-T{««ÉŸšoè!Ò%mö‘İ.ğ³k}€DXìoê2Ê$9LÃk¤PÕ+È}„Øµ–ßj^CĞ9,<.î¨‘¹ Y~&„„'XÈf“çáT$šâ$~:È¦YÅµWëŒ<“Ïw†kl˜œ~}È?§a·v&† «®S…PEN~îrÙøJË`Gƒp*²ê‰‡4 k‹µÛË2·´.ÿø«"&BŸ‚ˆ‚¶QÌdÖ+QúJÖÑ ÀÁ€SÆTî?K–¬nÏ²V=9Ş€‚öç>)R5éÖ!rEoÇıG€¤ÒKˆU=‘‘ih;£kÈˆCd€» ÙÌ”ê¤æ›‘tìe¼'%‡Fmsò¬j‡Â§cb /‚´:ÚJaÃœÓšòPêiP¿
ƒ?·à±Ö¿x_|ƒcıXoX™ƒD’¼‘Ÿ¦ÌÙ^å¡2—„n¹Ñ„ÆóÎ6L.e${x^„f
†)–ğª~Vöşœ*$ôÍ³L†G‰ÒóT´¸;–ÏRÀÍ²Ïò†j“I'uß¥3ølÈÈ—jæ$†Ú]“9ÒXFnÆ\V9Ãˆ2†îX+g×0£?º¢[á½RV‡†#Dr‚RG¿–&2¸6áæˆu”ÏQó&9ºÛaöSõ%‚¿ˆO;¶Ê›Çe¬D]sÜıb¶ˆˆÖÑjËˆ\Å£›»nJ†²pëÅˆó]K¿‹ŸôMùƒ‘ÿj:WwûTl‰‹ ¯_jEóË’LÜ'Ràú¼êÏŠó­Æ]ìÊ÷€Â‚VG/‘¶~Šô°˜*nõ+(ŠÜ:Æp¯Ş‹!+dà€î†ÊÍÍ?‰a‹ßšø[ş)py‹ĞÇÅg-Œ1¢:Õ†‰£MòŠ÷VJÂ„8QhŒÈğÚº€€êzß^í%O¶û&ÍŒÎAƒÌİOĞoÃë»EÈk«¿µ$–á'Â¼İŞ¼SÿÎûbßq§¥òîHÁõÍÛ2.tóï0t_üéñ0 ÔÜÒ‡ØçêÛíQÙ­Y®K/¸v†GĞå}R6¢© Õá­p¢z‡vX“’RVvŒÎ•QĞ*ü¨dîFÙ'ÂŠâ¸Sztx!ÖE±KI29y¬¡ÖÃ6A%AÔ}w…ålÏ½	¿-AÕ-‚“â€a/ 1í^`¡UG O@‰¤Pæ¢,0Š7¸?€é×™2ß®%ª	!wî€8Ò³;)¥_çUİÜ
i½½"F¤i§û˜ÆGé÷
÷¬ZÁZÇ®8xƒ] g­­‘¾±C ½ß$ÊŒ ÿS¦‚{’“­åäòì&¿-a²¹ä­#+º’xE•Aí~ÏRa5B™T=A“hŸ`1§¯à J$!©_”ë±pşxHQKú=EÕ¡ÛÖUµ“–s´U<1ÇÍOòbœæ=:ÛE–‰µ4»p¶ ~ÏèÂ"Ñ¿A³–àêı<îYrmD3LL0"ç—£J×‚gµ9Ò1J9K„lgİ—ì#öy¢@îH¨cğæ	*óHï˜Bİ¹rÜr¤)‹"añ¯“Kàv™˜š³*³Õûş“XÂùJğÇĞoD˜ç3İ÷2,Jˆ\—-ò¶rü-KëšF‚³iÜz/˜~r@m)=ß¢šQ¬´,Ø¦‰xÙLSÄ¿;æá¸›(t·55Ê€-d ³ÛÃTHÉ*Œ›šWgİk0`â×ÿìã‘ ü¶!+›šö—qf–ˆõJï¾ŸI[¸0£ú›×ee¤|ˆƒ&"ßsËÁ\L~BW›å ’:>A$7µ%[^#ÈCœ,°9‹!ıâ²"pôß¾=ù7v™³ëÒÃ°à0ãvSØ3Gn/q¦:ÊãN;äJf9#Mïn£ıÁi…:’¡WGgt7¯ë=Húˆ}Ø«Ho·‡:.¾ææ‰ˆÄåw”~4XUƒĞ.‰›‰.¯°ŸÕ¡ ]^Âê˜“j…ºª}:±ŸócRî>ÖŞıóá“ß?r-F ‘p3WyLÛ tJ‘Ş€Ô¼|Ï ÅµË¸˜Í´¤ä©Éü‰9!")¡GJ’+Zpy!ƒË¾	².¡®%üAúmóuâ‚¤~8úÓë¢ˆkŸ)Opäq›dqWc£·éX¢&J$s÷@§-mcPœ˜¯@¢vp´ÀÍ xÇ?j"ÅïCñGÔ¢T,¿&6Œ<ø6
ka…
pÍwT¢Çóål<èäy†° ¹$‚a½°³¢Ñıçj…|0¹Áá@Ãÿóÿ{wÏ££Šdî÷.çà‡.ğ¢ÍÈÉxqå¤¡,2Š^¢]®nò/x ê‹¤@Áæx)å-­–—ó…öwüŒ¤Ç°ÕóºÒªÿ®ÒPaÔª“¥U¦W»CÁO¥_ÏV×g¡œ¦¹3!N¹×gv àKËMtK}-ı=§p‘2e4c…Œ4àCuIS7›Ü˜§ÌE4ÕûWhgK¤?Ò%]ºà ¨1î¯FÒàá‰Ì	å})æ´ª,§©4ñp¹O[É0cwOß%1˜sc‰ƒ©½CyÚlAñúŸ®»ZmÔRcµH©ïç^ñ°”M¤òĞAæ_?Ò|Ìªa\i;¸ßì@¶¥0¢™îÇ³Wª§¢V]RÄ–JÏ®‘×™q|Åª¾–NøŒì‰êl`a GQÎÈ¬‹hÛkf×A‹@“dWºï­Æ“{ŠÒÜÏ	\¦§8’ß®=¸Ï#°ËxŒ-&˜±‡¯Dà7WˆÑØmª!üAÚ“UoF¯ù‚şXî*è‰;\o4qÂ±)‰@"Ôäáÿ£ÕôƒHpô X'1±îĞUU€ÕH½5kNlŸñlû‚›²8¿ƒÇÎ0ˆVPÔ¤_ÆAh,w²>;f>fì’¦Ï]L}"­²Fµ‘9¿p¬çÌP|¬ìøÆ®³G{½’¥¿Âò<ÿtóJ#¤$³¥åS'ÿ©Æü2uıèØ•¾³ê(SBcæTXi9¼îE•ïòIg´ƒwX‡7F•Ïeé oÒ>qù¶YÂ`VYdŞşï·5‡	°ï¶v÷¶Ğ¬0¹ÉQ
®ãMÊ=–¸J49çÎ|Á#~“¡ÌĞ!¹¨*¸æÂ¯Ìú³Ìy˜]¾ƒ¡ˆoV¸á¹XKÏÅƒ¹Ãç1oÊ"¾lQÖ¼—¹”@„“8MÜŒ'®ò‘÷k2¿'ºĞ^&!AŒ©>‹ñ|JŠãÏótºôf…sE©f@Æà9Ä-ƒí¯‹ºt!è„.Îq÷»»"²2v¥^‘º€­ŠÔßtßõÕÜ¼TZ‚úïıÊºÅÉQo ÍÂ;&©ÕbÔ>íá¬G»›Ìi‘–uXö€Gµ>;ùéN¼Ìœ"Zuı„BÑD0ßV¶İ«¾`R“›>£¼ ´¶›Z²s®¾ƒ©Y#İ¯¡«V÷0ë¨~y¥(¸¿ÓŒ¢¥™n½úšÕ[º¤ıÀAë¥=•Cp|ñ¢5÷Î¢“NÀk‚)¤ëQœœÿ#’³¨(^SÁp†KhrŒ¥=‰Aj1ÁsRPÌ£ˆÚ`ÕBkm¿ÙÂÁÈ¸ \ÊZŞD&;J`­–nÁÊÿë¾³C,½k:;ÏY‘äÁÔ!œ<O!í¬}s 	Îû§ÁÜLÈRÁ#î/T•®û“2ÂœáWì5ûğÆè·ûI´ØÃO2ns
eïM-›	:FWss˜ÃQ¶'ĞJ¾ÔğPO>éµAÆ»Ãe |f>±ô#&b1Â"¨(¹—)Ã‚å?…^.÷$K~:ü£Q¦båÃ£Ã(KÜ=ë\ïïYÅvİé1ÃĞÖR²Ø‡‘ˆ}<jÑLZÑ+^Ãå{òzÍ×@6‡o}7*ëB?.´Ä&´Í•c®¦‡¸W}}y¢"HKÄE­|³
.ÍÜé”Ì‰@ˆ–QÄhí3é·S ÓE “‡*(ÅI¾9Nr2 AO.zşğŞÅ;ÙTí–î>½ÿß¾6×}NS©äÅW’ñS{Es¯¤—¹“òr¶w™ÅzÑŞµä?xŞ• z4äÆõİüä'Æ3
ø¼Ì”!´;…QÕ#kÀŸßÆ>ÌØ†·?õ½9[h4^¬!ô—Æé^5ÃÆËì†eŠÊ8*eª“¶+ÕÇ#ncõù?ıÉı%ÕyªUOÏŸÇ4¥5YúkĞğÇS'=³´€*‡ÈtŸJi“ÖÍK°9h"íı2ÈËÔœÊQ£„r„ïfb^ ƒÙêÕÈØõDÏ	°p–Ã¾ò‚ÆŸaÈîãÑû˜ƒèIeùÑ¹qkxÉ¸k£˜=zQ:¤ÒÄ–áŞ¬WÉÁ@0EĞ Œk;]ãI¸e9< Ê…ËÇóh^¡•ïB»>G7ŞÊ)(üÉ}{*Î(½S©“#n´Êyk%`!r22Û1¶ÙåÉ¯ÊàÏı!§6¬fºÊ †k(`x'CË ıÊ«ŞÏ —êàÀUït•õÇÌd}!ém/Á+À÷ü°‰ÿ³bŸÌéN˜¿YlÔì‹g%@t¡tü§I¾Î&@0‘Eç'µ3Û¡éàp5ÕÿÎ¼—ƒ8ôPæS·â0°}ÎÁ`wóó¢ö@Û’<7æôÏ kUNÛ­UÊÔâŞ€²5Ï³Æ×ä0[²e1¡ÄÎ%•­ZÏÜ5Y N¯Íò`¬éÓ>FĞÕtÁÃê‘ ‡âcj¾£˜€ymĞû@ßB×òüúÇsÌªˆ.76SÑ4åK2m	Å6½˜¡’ÍÑ‘?IÎAe`|Y½‘>å7ÑšÚD\ú¡JYhnSò¯mUÿÑ²ªCÓ=€H½QD|ÎNgsÑØº{»‡cUV…˜edÔ –­íYÜÑÿûà¢·¤#É§Å	Y¢£S›Ò"ìv¼Ãø˜J‚RIµm6FÒ>6Y¾âå/O€áÕe¦I¬?SÒ§Øe?ÿÒ¨l£\İıøêÔ ëŠÒ¾°¹³•8ÎŠ~3%vviÊyÒŞù'İ,Q¿Á8µ´4²Ñ• Ós‹Ï°å_İH¶Ù˜èÇúÁ¨ÓÄvôeËú\5÷šlnMÓõ»t«4Êy°.ğ]ÚÕ1àÅÔ‘l=zÛ~§*$eÒ«€¦xÃÏ=Ô¬E•A¢IË²ÈçVkÀÒ¸ş%—ÄÔ°™ß<W˜jÃùîód'd2Ö!Ê|–4qvI~kIù‡©;õLÖ}b‚=Méô%7·*bÛ‚9TäÖ¡¿¢Utİ`®İt¥÷L2²T×±šƒşF©Í@ PÔ;×:“ÑØioõ*÷ş×à¸[Çò×Š*2Ô‰ı™§FÜ} °·è™(é×Ú-ƒgÅşæÎƒyrC9wJ¤YØó—|`{Wè$KÏ``tQ D÷ÿØö2Åùišè…&VKÛ*Ûoğ^.ÙÃÕfØö8¤uR©ò—ºµÒ¡Ù–Rİb÷„n,	vdRL*Ì÷åHÙ±¡»(wÄ,q‡Ò¢—LÄ£YšÙÒo§‡uÛÚ>‰³ÎóÇgÚ$ÉH
¬sã{@¾(\ä$£Î8¤Ú|·z«Ì’bğ?£O´ş¶ŸÂÚŸºyOäŠÿ0ßÌD÷»
â¤iFÛV¤ ğ÷’2BÄºì·‡°È‹ÊÜ>,c'Gu4ÙğuŒ¶SKşœÜœá|¾Nö8F‘d ™°8¤ÜÂp÷ìK²âŠfN»†ş†	à°İ›£DHZ.o¦Øë…%Ñ%ëS#Ş»:b¦Gx‘#ó FÈ>×Şº’ƒ˜=^<s…Ùq®½ŞıGÕ±Ša-EäàaÙ~(b<õßß¥†—t­úŞ‘+FŠ¹hI¤’«±ß@>«äú¡ë]g9Ü¸˜`†*“÷•ß‚–óÇ÷€HÆ¨®9üyà'|¬Iş&ïV·ğİl_Õ+óà0bæ®²@ÑŞCbX
'òÚzòáÃf´½u³˜mêÕ r~-,íâAçÁvgMÒ/•PwDò	èâÕH?\€+®Iòÿù¶ó‰7âğ²ŸfÇëÒ$TùeJOì,7éãäZ šr7üK…·÷©$)ºÅväğ¼€œïBD#Õ¤š@¹©4ä’½=±7{ÁdQÃpEØÊ#˜PîäÜÌs  |s5$OYêæÚ¬—úåø×æ”Vl@{À‹TtUï–@'æÛ°"oO¡Fá	W’j`Ì.æ´‹Æ#şL#…æïĞ†ÁA1›şæIb©ÉğF®Ï/³z|£jÀ–Ö æâ›²ÑÖCK‹)®wZØÂäŒS‘èg`1:q
Sú+xÌ é	¤8Ûé$c¹!ˆ˜„¯ºÎuàV"•‰%éşo¾ZDÒÚ¹P¯§Ôˆƒì‚HêÇƒ˜¯‚cŠg¾ñäÅ‰Ép ÜêæçHìæ­×¼Ú[dQÄ…²É×Çë‚>
p’Ê\aÒìOc7x!ë¥%1­ØLÃàqhŸºôÛOÙËì½ñö·§W%—\÷Ëé¢Æ·í-ÖXËï°ã –ş ôƒè[xõí°?bo9";2=é”æc*îªy¡m¸ƒ™Â–¿ÒÅdÑîe†"ãF&µQyÉJò6øÀÃîiqé’^(s9\`ãWÚï;´n4ÖBĞ‰:–y±Ï™Ò)ğ‰C“SºØïAç`A²%|1F‰ğè‰³ÔÆHØn8Dùj@å¶€ñ,hß÷Şš?Z$ªÂ¨xJOñ£ÿQUî›6ÚõÑØ9h"( ¿ˆò å,ù=C{/¶ÿ;Î19ÿ÷„¬òò£8CËº`İÁ]²@¬+ä#­óæ¤¡n½É)¢TËG^:ˆÍô3Â­ÇgËü@®Ü
ñë»èÊnôº5ac±×‹€Ùw*Š#˜€£ôåX½’Ä
µ‡%9‘7í
®ôğE«ø‚g<Z¥<rc0»à%õ7®İM1÷[<…UÌmİë'ŠõÍ+4Ì¸}«Ğ;æKÂúzA ö8`·?ºé­ƒÔäÆòî6¢56öí«¶'uO-“>p‰ºÓG‹ïKha÷õ<	8ÿ®k´-Q#ÅåÙ÷å­Á{;rrª3yø&‘UÇûN÷c eMqÚb¾›|A3Çy/·<H÷pà[· Øx…y,ŒLtjüà÷èZ
ÊÂ8ËF>Úfñ~†eø‚«¯z<{ws– ÊŸL‰ùŒ0m¤ÿˆPØR:ŒW7ìDëª¦ú‹‰á¼…ĞO‹@’.w”Œï˜Éú»‰LÍlÜÉğ¼ ı2gRï~û3©Ú–#Z&ú™8ÇCkş'ŸûOËŸÆ{ŒÁÃŒ~™u!û¤ÏŠĞeÂ[…f¯;”dEÙ¶úü¢ü‘ÌÛ dÒG,õ_¨nàğıbM£8©‡|öãF"McÅ´ı¿˜Ô7EÅd9¹lÃ©óR‚¨ÿş|ÈÑzÄÅšÁŸTh( Æ=BÿwÏ[6€W§äèºkš‚ñX}ÿØGƒô½€¼2Ù~mR“ıÚø–­ßŞ"”·Ë­¾‹×|è ·ŒhÂWe0@â?ÿ÷f¸Ü.xGœ<7²dëm~3–h«ŠŸ¤¨Õô„¹>[¦Ø'ğr¨'Òº%j:§’¡æĞ­¯Òí_S¥M[×I¨È!şiÕåö³ÏrêÎ<å€È%ÙN„‚kœÀÊEÏ¦ïS ‡¼9eLN†Ú¯>ĞÎsY£nVo@pÙ)jSWŠÈÑ¬LüUôôÕ7*v|áË^Œ<#ƒ	bÏNŸ5°œ»]†øğBhz&f*Á'‰ò·…¡Ø¦šÃª
á3½uµgÍ@=³’ÎŒòQ&‡ÌºÿÊw¼óïÆÙ"AŞ‚g/WşÌbÊ¾S6ÿ«K}/Rœ$¾Fu›’@¶dX“•—ÁŞ"IÊÍ¹RÙ;Ãİô×/”ŒóÇêZ8ÎÕ	Ñí¬ğ‘Vñ9_‡6ÆcÀ!,§cÊBÏÂ	è[X©fM şˆw'N–'şÒ´ó¯jEÖ²tÈ˜L‘Yq+ÿı6¼]’é×RİgªµÏruH#/ş•D¦Oö.Ù~ù¤Ä:Gìb„¹'s£§<sCaÁÙ6v¬JÈ¥˜?zt€3_H«@RMé‰‡™ãO_Ô`Ü–š#xE¹ó´;=ÒP ¥ßğ¸ÍÆE‡&hÆç2d^éÍzháÈéïÜşIáOİPI¶›£UlCşuT‹Õ‚ùP\êóÌ8YAa¶àÒ§ËnÑĞf–KX^46Ä%NœUÇ-.Òãá(Zw²E6!W-ÈÒÀ(ı¢6ÊRFùNƒ´ÕËR¼‘ÈäTG)œMŒKñ2=¢.%HñK™Õé5ÔïKÏ®<äŞ:¸™CÍ²¡—@æ>{†ë¨¤’¤7íhc¡f¥Æ|Cb&µ…C3»I_*@P^Û‡ó92….\`p³¬‹îø‚†XYá«JQ´Kò»éaLu÷ŞaÅ:¡“qV8µ€7m~%µ-„%˜Øär·ã”b×¡b'“©³Ş^ÌÀpW†'„ìk#ˆpeK—/öoÕ·Ş«Á*ã+¡ò+q~•7a]‹ÃK9İKiKíÎË÷ñ4ìUÒf&0:"jŞ¯û ÉP6^§•<Ÿ 3ïÜÏi¾s&ÌÚºˆôÖH¨UK¥G¿¬N	˜³mc€L#àEDÓu&w	Ííuhüh0ÜàšË”¨?ïŠÛqX—i$¿U_` ­|a
q=Y}w„«køhºÓlò¥®{’Wyvxm$súª­ ÷]’_5ºø˜>VR­@ºsü‰–,ÎsæK˜R©åcÔšUTÒ,„ğB§¶É!hj„S›¬CŒ¦•œSU³Z‰Y•à€ £‹ƒmÀtğJ»‘øºTY‰”¼ğníN(d£mrp*ó½t¸qÔQ82Pì(JvÈ0.âƒúª-ÙËn¦=C)`@F£ƒ	Reø¯y‹‚¦à9’ïÅ±®g@²Eá…u ì5ĞAòß?mâ4„Š¥—Í»A['±ı!'l¦•iFúgõv©Ü 6×iÁè€£RßcUOœE¬>šõş²¡ı™‚kõİ!{Ù~Â·¼b¡Ä¦e6u­<7²ĞÌú…öˆ‚qëÃ„±V+ô@4îÎ°iw=—¸˜Ün?d¸	è‹AÆÅ¿iÅqÇµ«	.úI¤Í¹å%|­ö`¨"%^ß¶léÛ!İÂ¶z)®tBÅßÿ¦	µ/‚ò­µXË8{í®¡h<Šk+Ş®´“éAø˜hä_”"öEâI¹uH$s*­*´ÚT6}	¸÷ÛJwî.Ò\òß›hşsO£?ó¼ÕyÉ© åâCÕEDÔ'çÛ_¼rmã³³[:	'¶1cÃÖ
ë‰‰ÃÑ27½9Pö"ÈaÕ	`Î>ö¸3FÜóønôGJÏÎÏA5vºâ¬İ>MÕm£Ïxã. ¹’4’²Ş¾_0Nh·`ø*Ax¶\µQßü	Èì& w/©â¢¿p<íäM\1@}˜êÄÕwy“/n¾¾`†¯ğÄU€°±8—Cê¶DËC²ÒÿGºª8Ô€~»D«×ûƒ'Ÿ³Hİxƒ$'Ò¬Â³çÊ¼>O.rİÿH@±bÔ9>Oç7u WÖ§¥…˜³™Ì{Šl¥-vàø:õëÕ÷%Ñz(›m2í	WnƒK¾‘„â6×"<8eÊ9£Nøo°7a4±Ê‹<Óì¸<Vì–ç#Ó–}ìöd¥~¶¨ZYôÕ_Å‹yFD@ZT¶=²R}z“”&ó®OÒĞTd-İ¬r°¶Ã·t­üô«;›õúÀıT©JèJfkÒxµŞHL W-eß²{‰„şgÉ9ü’~»3Ôºà¯®ñx½Q>SmKeR“ó‰‰Ïªî¦FâÂä°‡§½ã*­îËHU¼ÜÑd—eÓ0=DÇ„¦ú «ãêÁO?>·—œu-a†aŞXŞµQˆn¨7q=³²Rw÷	Ñ”Ô€[şTK#=F0TbèùÆ%¬/é™ÛxÊ®j Ğ–,a—òkºeêNŸÏñÌø—kG:˜ˆCÈc”º¯ª”Æoì”C3ü2‰¹ñ,Úq-$nv )ê›Óa†;~„¬ŒÊÁÚËN<×ÛC&5(·€Äú¦–4^C»q‘È'½"€ãÂZ¿Ôx<TïGîi$íWÛXJÁÅFç9OQ9Ì©)âh¨Î…˜[
š“ï4Ş,LÃHÒsÏ.h
}w›î«ÈŒàf¬¾ÙÁ§³UCê4 QsÎ¥á:íûŸ˜úÏÃ(YËYaY0¾æ²6 @p½ˆÉ®'TÜ0<=NølÖvÑ@uÄToıOj‡ƒb:1ZÜ7« ƒ <Ú ]o  •  «À +! :n ¾¾ ,[ y6 b4 Š¿  Ş fÊ  (Ó á D  4  $Û f { 0 1B G F, U  I  x  x fg Iê  Ú‡ d  ;  Æ  Š IN U ×  ¼ a‰  ü ez —“  ’ GW ÷Ğ  å  ™à  Fu  B8 ‚ ÎÒ h ü¹ U¶ ]ñ AÌ   9 ÷¤ ú@ |¿ –õ  H7 Y„ G  ‹   Ïé rƒ ;  çı I İD Hí *Î 
 %J c  ß¦ €y aÇ  2# i¬ H° Së 4` •Í  0 Ç² 6l  õh  D¸  ‡  9Æ „ „  2n  I;  L  €† £Æ gÇ 3(  şK ùé dé O% Îl :Ä ¾ò  :¦ ÄÇ ú ¹ ¹¬ 5   …P Ì `1 `í ·	  
‡ ±k u3 ‹" I«  Îƒ 1 °  J±  Iò °q YÏ   ßC [L Éx ®5 g, ÁB JE â aõ øó ^b  9w  	 Gl  _¹  OÜ  ò   ½ °æ Y“  ÿ# DM  İF Å¤ gØ ±* \ø …¿ Ø gk _…  õ 0Y ºº Ev  *!   VÛ  i  Kù  Æz  ~Ë à 4È  )³  • 6Ì õĞ X K  €½ Qr Mè u Å  9%  å ÔÆ  M; [  Ën G& [N š  7 û  =W p  Õ-  3  ¾ œc Fú ‹T  ‹l J? ·§  >İ  ÎÁ  è8 `Û  (“ Èî Ã° z÷  2> ı‹ ûÂ  †ÿ 7= €Ï  Š lé ÚÚ ,° ]ˆ b m –j  ùü œæ zS À[ 9I cœ  T. /	 =Î 	ø € ?  >l ¦< Ãà  âı  fF bÄ `†  •ó rÜ "% 6d #-  àV ù” r( ><  à¥ VÈ  ïù  şÆ Sò  ÕŒ  Ñ‰ J ¶ƒ  $® Z¬  ŠZ  7Ğ  Ş  ä ç  •Ğ ˜ı  Ía a ® @Å  å2 0… ?Ã ‚ K£  8 *  ìl £ä — ¸§ ¥é c
  ôš [ Š"  F=  Òà F DÛ  )o F 	 zŒ  J÷ m  ì6  Í6 tÂ W ]É Hì  WÅ  ıé ]¦ º gè ¨ UÊ  Š©  éu zª  ïc F üê  DÖ ±Ğ –# Æ ¼î  ê  5 RE {B  Ê   Åq e  ^‹ _ PÇ ,İ °§ İ  J Ñx  F` D’ û† œ ş	 Î –£ ]—  Šw  4  D    <I Hz  ú5 ôb æ× ¾j  ı  éĞ  *F ( -   æ‘  ‰±  íp *û  F’  Ì¦  }  ;ä  ï´ O  \º ä¢ ^ä g˜    /r  V ®„ ÷  æZ  B‘  w  Lz ¨7 ãÌ 	W Ÿ 1« 2y J :  .[   @ ¸B [« 1å KY dQ D\  KÖ $ Xq  Ó!  K! Ïd  Pd R  à
 +ş 1i  ‰Ø  C  JÛ  J  C; ì xÍ ¦  \  ı® +œ  š —/ >¶  õ¥  ?” M­ cê  lr äó ùV  Š* Z a¯  Šß ½İ  R
 ß  
 ËŒ X ÑC ? ) Ş  Ò~ >  < \( 0´ al ­ _ ­Ğ F  ä A‰ Í. Æ )Y ıL j óå  ‹) 8ä úe -	 C  X8  {# dp •6 ;
  ] «• â  şˆ s  Bˆ Uo Éü 2û DŒ Şv [x Po  æÍ  í: a o´   :| ¹Ù ]G yc †- J  ‰… \¬  *k €Ÿ ˜º Õ;  1ï  I
 ™`  Q¾ 0P  {M ˜i mı ‰  D K g ¶ Ÿ   Jq  ˆæ 8N & ?0 Ì ^F FÍeëí,ñåV«él1Zn¹n›o¶ZUã|?è¯<*Bí„› NO

// .git/objects/pack/pack-8f65ebed1d2cf1e556abe96c315a170d6eb96e9b.pack
PACK     )“xœ1nÃ0Ew‚{@”iJ*Š"C†^ƒ¦¤˜Hmª2ôöMz„ïøoôZç…2&ÅYË‚J,M|öSEâ4ù,A²û’^÷Ä:7JX(D¬!qbÂ¢×äçP£:¹õèp±›Á‡ô)oåvZ¥ÿÁùº‰}ôØŞãDìC/>zïëfcÔÿú®U¯Ğl·ïÕö+<ùŞ+<_=2LeØ±»_I°PxœÍJ1F÷}Šì…¡3Šˆ —"¾@¦ÉÜ)·ÓJí,|{{U¸àÒ]ò…œœ´Ê49šô¬Yb¿Z=·8sOiQ¦OühÙŠw¬œšÉXíb´—*(?‘ô’Ãh&µ.ägT^x´­TxŠçÏX?‘"ÜÑyØ°~7§cBÙïA9cGo5p#”¢§{lÿ»/VÆvH§×—GX»É^èHå"¦3Á¥ê?Å€-–,ÄÛ?àç4ÄÒA|E„’[-)u§+M¨á(ôğ—÷ƒÂoA”xœMNÄ0…÷>…÷TQÒ´Óv„€8¶ÓF3ıQš
q{:°aÍÂ’ßÓ÷ü\²*úABÓEw	|aîÕy'ƒÊĞ;OÌ¡Z»6ÊºÔ[’¤‘Ø[i…·MK}ÉŠÆ(L@G™ÖŒoé–ğòIÂg¹™‰òxgJwÃëü‚®ówµw>ÙÎZ8İ9•¢ÿÍCT*W$|lGV|üğ1¥¯cZø~ˆVğg0ë˜ö³»‚Ú O´Œgz9‘5kÁöısÍR7xlBEñØ5/4+|¹¹ml›xœKj1D÷:EïFıƒ	Yd‘ktK-[™/r‡àÛgÈEÁ+x‹’Ác1D1"R)y
SÓöZÓ\#×˜BneRŞ4‘X¹fïĞ[ç©pfï|hÆiÇ)•”|&…ßrß|ô¹Ã''Ö×:_î8şàı¶b_.e_ßÀDk	)Oğ¢£Öê\×.ÂÿõUc”WÀZáëG@ö™·³ağãØ·ÃmàyF.‹ú-šSÈ‘xœM
Â0„÷9EöBI^~šˆˆBnÜxƒ—¼Ô†Z+!.¼½Á#³™áfjI‰ËŞ£ô.:c½mòZTy™ŒD	V(C’½°¤gåÎê¾ñ`RFKtpÒ«àmœD)@œ&`ø®óVø—Ì/X>H™héf,?sº¯˜]ÜÖc;¡À:° ùNôB°–®¹ÖôoŸÅ¶ö‰øm<×±[‰}ê¬FÄxœKJCAEç½Šš¡ÿqÛ¨îª6Eò^‡NÑÕtïá¸k2CÇ\s1‘Mm9öİ„\½.ÖrJÅçÆ½äVÔ'ï´MÁøè=a+ÍEWˆQèÚ†BŞUó0\ï
?×iL8ÊYàç’À'œ¿ãícC¹ÚØ^Á$gc¶QgxÒIkõ ›¬ÅÿíUg\Ï€Dp‘
“o×±ßp'èr‡¿sòKÆm«¶ÄS‡–xœ;Â0{Ÿb{¤ÈÙ¬!

®±ñ:Ä‚ÄÈ1·'â”o¤™×jJ€FSĞ¦)Ç`<¾g{#ÆHV½¸¦µ„`ŠXœœ'›0êà¬%ƒ‘cœ\2ä{§øİæRášn\?,Nòèf®¿q¹/œŸ],Ëz7 õ¸7à Öj§Kn-ıë«¸§#°H‰ğ*ĞŠ”M}I[I&›xœOANÃ0¼û{¤BmlÇ‰“
!$8Ğ'îhc¯«‰9¼´E<€ÃJ³3šÙœˆ@éÆj]’pÒÈ
µ5X[¥”ìÊ–ª†ºÖò¦Ö’Í˜(d¨¹®QTV¡ªHò®´	!*tZ¨†›ÆR®•×<Ä/şìáÓZö|0]—§~B?LœAèRnÓr÷\sÎ6vò9Óı,‘C“cºÃyŞa™GŸÁ­Ád¼Ñçé™±÷Á/p‹‚+
}8²=œ¦y¤éòïŸÇ›æC.Å	zŸ‡µ»¤vk¸n‹ïã¸>¸„¿)±‹u™xœÁNÃ0†ïy
ßSÒ¬M3Mˆ€ûä&îb­KJ’iŒ§§cBpæbûÿ$ËŸk&§äh¤²}·ºŞ4½SNYl•U^:í{»Qj%fÌ+h©½³İ ±“ƒj[cj£Œ•ƒìT£I†kH^øÈğŠùŠaçë€ù;<NÈÓÚ¥Ó(£Õ6²máA)ÅBO\+ıw_Œü±…¥p<Àx®rŠ8q½Âİ‹?ñ†ö.yZÁŒ¥\Rö+p/_î]&¿tÆ©¬ Ó˜©„}MGŠB¼.p\Z¬Èj Ó4¥Ëí T¶âñGrNù7z?İÃm,´ ô|rpf(3¹¿,Ô:/2ïâ£r{xœKn1D÷>E/!!Ûí_GQÄ‚E®ÑÓn‚Ã Yp{F9Ë*½Ò«ÑUa*1 i¶(šEœÇ¬.²©Ö»‰¸x6wîz0¡²/ƒO¨)†X6”&Qu%DŠz:ù@)~ŒóÒáØ.~¹?¹6ø®—ı™û8üÍÜ®{Yæp‘µ‰`g³µfkç6†¾»7²¹õ£êıº<çí÷úù\+¯+ßjgó‰JM„—xœAnÃ E÷œböU"°Á†(ª²È¢èÆÌØF±¡ÂXmo_ê RW_óç¿ÿKf†ÇÁ{Û’ª:v†\gÆÆi¸!¥=qc¤˜9 ï<«Ş¬YV@šÖêÎ)Ùwº‘ÎY5ÈÖ¢¸—9e¸‡G€7ÌßH®ô8Ï˜ã6­–³Oë+¨¾UÎtÚIx‘½”¢ºk(…ÿË‹‘±\ ‰`IH@Xö-Ä	¦tÃWÙ3oB¼ÏaƒçX•X0D(3Ã˜–%}şÆıŒqâí"N¶=>NŒT½ú¢å€ÜXü -.wù›xœËMÂ0@á}Ná=%Ó¸!,¸†»%*ıQsûaæ,ß'½ZTSÆG"ÇÚ·ØYÒÄq˜”z
ÒÚNÌÆE—
èQÚèBJCƒ”Çˆ}ˆì{nB‹¿ê}-pÍS†—–'™w.ÿqgÎCZç3¸è];­5s­úío†ü>BZ—!{İÀ‹ÀÆiâQŸæÒñL9šxœO»nÃ0ÜõÜ’-[vP:ôº4EÅ„2dAúõUÒ­c§#¼Ã]NÌàº–LÕWº%­¶¥ah
ÓéÚ!»–p0•!µaâ5C=P¼%×‡Øtäú~BO=³3Ø¶MN+¼æ1&øIàÓ½À«Ÿ#¦çò~YPæ#ÅåŒ«Momg;xÑNkUØEræÿêU`Ì'@ïá1]C|ª”úe‡_ÿkFYÿüÀMò²l3/¥2f‰+\–öù¾ñ^|Ÿİäûy:Sô|€÷ı“? ÍRdgJì
Îû‡ÄûxÎqâUı .}“@xœ}“ÉÎ›X…÷<Åİ£üHé(ÌƒÍhl0;l3c¦¶áéãNzÙJ-Îâ“T§Tg²0™¥Ë?aéŒcoä-¨,XH§wšxŞ±t™Ë~JÕTÀH§5½Wà;Íp¬ <~ÿàoåüsAÙ„¾º~Ê†vı*ª¹\®_·şñP¤v;(Pà$G’Ø‡>ªyÎ& W³±\Á÷ÿl?ÿj+†UøöïHªn:ÀÓ=p4uGOú›c /¤İ$Q”dQô%ßêœw1ÈL¥>·,ì©lEQ,6Û•ì×s;áŞ”5Ö>ŞÂ`ˆA(‹%áö²Z®äyWw‚¡ÔÛ`&¡Â¸»D{®*läxİÙ²!IP9G©>8x¼¸ EÙ¼Î£÷«®á8Ÿ1Êf¬5M ‹m×—IdçJ«P-™¨J¢ß›ÒZå#&NØªpa¶*˜F:¥ <Ö‚²sOF»êùtVô<[r\Dy°MöT»Uh$¸K‰#ˆ)Z¹3ûÎò3öY14÷Éã/¹[‡´:kJ|9ùüHõ‚%Âùv•Qæï‚‡–Bü¸1èSîw~‚†‹yÜP¿§´­WI``ÆF‡ ç/ldgö›Ğ,6ã·â}JÂ€dZÂÄóöêqgÚ´Ûdârhºt+~T}™0fÚ/Ï] Şšc¥É7ÑŒ«`@k§ÃÑÂZ[#¾Q¿_:ÂgyuÈMøXUr~²·0ã£’óŒ]·ËKØ3Äê‡ë'ÅCˆJ–şÃ*g…g¶\ûóe°6¿Û"‡V›‹O;ÁL8+ß{Ç{áÒoQF.QÆŞa0ĞŒÅ½ò/ïù­†ñD¼ƒóÌ¦‰1œ¨¸fLÒN¼Å8 WTWöŞN­¦K˜X±i\ªğÏ’eH£×Ğ³IF_î­
p?6¨K–ÙCfLtEsìÊ½-i¯YzÒíLì|ñŸÊlx¨¥k]’HW^çÕYÅÀ?a’}Îüû÷UGùÿF`fWÍUÚ‚?Åû6=B¥+x340031QĞKÏ,ÉLÏË/Jeğ57}¹¿ñÙÙVg?úsÉ°¯¡ÅÎªª (U79?77³Hå¥e¦ëU&ææ0,\§ÊûÇñWîçÒGM’Kêd-~]~ÕâãéìêìÊpu•³Lü¹#)Ju®j%ßj)øUâ›˜š–™“ÊÀÿòş"ÿ­a3ª'0ìšõ°* Xd5TM«£‹¯«^n
ƒz²uÃ£m7s=>&ıvËä¼è, PH,(`0^è/©èÚ²¬#<aÇ}‹_KæmÍ˜‘MêİyH0!,2åŞ¿÷ÛMÛû97H‰½‡Ê‚}ÃòsâÊÎT–.-)œ–çßšÖÇv¢ %‰ÁY©ùRG×õÎgŞÜ÷¾¨›iƒJ¦ääWæ¦æ•3¼¾İ°]ñêæG%?iÚŸºËéª*?¹˜á›E‚àvû]/×6_yrì“à;³E¦féÔ¼²b†tQåK¿äİ¯ğÏk
;0ûëº¹{” Ò%©Ey‰9³Ÿ2L°š`çXÏªb¾U5:Nù„34”ÒóõróS„7íTÎÑn[=ù·yãƒBÉ[mL‘úÅ¥¹•Œ·ÒjTœÏMş]¦²+ı¢óÙE.oá*róÒ“3!1Y¸èk°ÕîÖvUÓ£G®uíYqOfÔ=2g‰¹yä>0Y[zç}\íşs*P³r3óôÒó†Oú\íZ¼~Éô“Å>m+Ÿ9bJAv:ƒÅ“'ı©şå$¾ìHyıE""[šÄ “Tş´÷õµYëÒ“·œX’îta ˆ÷âƒmxœr ÿµµÏƒ?·à±Ö¿x_|ƒcıXoX™‘ãñ6TÃ_v¾@¿’&]—ø”¬Øô¤¾™?100644 go.sum Q><‘¿h/ª<İGKq+*A±“
8ºt!è„.Îq÷»»"²2v¥^‘“V_Ñ)2Gë€xœ[ ¤ÿµµ°Ô6†îX+g×0£?º¢[á½RV100644 go.sum ¢vp´ÀÍ xÇ?j"ÅïCñGÔ“
8–àêı<îYrmD3LL0"ç“V_¾À%j½!xœmPÁnÂ0½ç+<qC¬üÃÚ&!íĞ‰ë­Ûz„¸ª—}ûœ¢	íçåùùÉ3X×¤<@Å±¡V\A5zW¬ÊÏÒhtnÛò™k¼)F©ì½œ»‡ßRP•b+Å—p¼ÑêåpÏŸbÕİ‘xVŒBïÕó"_©ïQ%§Ùpå¼’X¼4}K’Œ*5ÕÄ³º¢»ş/sÇS¢ ÿ¨`ı7ÖÍm"³zGqh°,İÙ„¢ešØÜş(àc}H-MxÆ+~ÛU‡`(œËK ½ã¤ùØŞÌîEaŸ=/ØOO¤ìZÍÕÎ:s™åïIû¤Àh‡`šŠG|‹ ÌaÒcEÙfÂNFH‚õÕrCŠo«µ¹åföÂpâá ½¯
èZ.2ãœ{çFŒ5¶ºÁñ´`´Dxœ}T±nÃ İó(YÚ'i;yl¥vêÒD]ª*"p±OÁÆ‚³SWıø‚“(cÛòÁ»÷xÀ‚(5m0w$´Ş¤Æì7Tàâ	cœ¸4Y†Ôt!Ï\2™ÀÔ¥‚˜Mï"T ş*È•±÷Ó‰…Âœø!ŒYJT¸x>OÒry•ùE¸ò0½óDæ‰UÌª§è1Z4ı‰›0è¢ŠYó„R$p…p…Ëä×"Ó=ÃB)P\› ß¡7n—`À_ĞuÛmb´È“–İe´ˆÜÊhyëyvş³7ÃKÔªİ%†oÈ3ÓMh˜ïF±5ªzˆìiœnñ†»GC<¿44ç'ti3öV0¤fûĞ µ9Äq {¬HÊ—pjƒÔ&åx°ı¶”´Ò¦VÎgô
¼›	"°nH`—Qw³ÂXê2‚›Zk4"(-ú5µĞ‘êô­ğ5lµğaîï!¡É…æW·÷\¤Ñ²¯>O—c„>¿Ó‰Ø×å%øşÑ#½Bxœ]RKoÛ0¾ëW=µ€Ñ=î¦ÄJ-Ì/ÈÊ²[‰µ9V`)òïG:i»`ˆä÷"SH¹kí,cKºNîĞGxlŸàåëË¤î·ƒÌLWÓ9Æj;]Îàôv²»+&3FÛ%°Ÿ¬¿‡¶7ÓÁ&=˜ñ
';ø]4ntã´(Äp2öHü>^Ìdq¸‚oA>è|{>Ú1šHz{7Ø ±·ğĞÜO³HgÍÀÜÔ{kÁÅÅŞŸ#L6ÄÉµÄ‘€ÛáÜ‘‡·öàî®@ğ9}`Hz˜€|&pôÛÓ×Î±NçİàBŸ@çˆzwXTœ×˜P/~‚`‡!ƒCßsÖwóY?ÑBã}E*—Ş?'qíÏÓˆ’vÆtW6+ş²m¤
ïı0øEkıØ9J¾1¦±evş³Ü;úˆVoè §«Ş[¡7Ã ;{_êâzÍ?q&’ïÌ '?ÍzÿÇ|FıL@S­ô†+²ZU?d*Rxà¾ØHUk8¡x©·P­€—[ø.Ë4ñ³V¢i RLu.Öd¹Ì×©,_a¸²Â°,¤FR]	Ş©¤hˆ¬j™á“/d.õ6a+©Kâ\U
8Ô\i¹\ç\A½VuÕ”O‘¶”åJ¡Š(D©ŸQk ~àšŒç9I1¾F÷ŠüÁ²ª·J¾f²*OñE.nRj™sY$ò‚¿ŠU!‹b4vs›LP‰ô8ş–ZV%ÅXV¥VøL0¥ÒïĞlD\É†²RU‘0Z'"ª™q¥¸±ĞªáÓEp„ŞëF¼B*x\)âÛğ3ûÅ‹TE·‘x­Wÿwâ6ÿÿó-ñ6²7{íŞ;^é–$á… È¶{Û=°¸ñ·ÊB³üï7#cÈŞîu_É‹,F3šÑÌg¤ÁmM˜ûj±œZnÚÿà³—îùö<f¡ïÊ˜)&WÂxëG~Ş4¡vš.DĞbğDêJ*€±+wõÕû—ì_^Ô‘âùRmšu+¤Ö|“bGñyj^kønÒkİ9M?šIm'óx2—‰;)´î¸­ñMÓ²ã¥²§~d×N÷V›Æ]«Ó›ìXÜĞ³CîGÖ<6ŒvÿòîªI½±3O.ûhD«íL®»­öšÀ$wqçn¼2‘ñÌDSø\X¨l7Æhr£G2Az=:‚)?ÄwşCh'0
œŞ[ÃÜ`é	°l­RÛÂÖI,Uá»TxPOí¦õÂ¶ëûŒfÎ9hOî‡İf§j.EÚ°Ñl¢œ¡ÙĞİAk42Öİ›şhœSûÃ±™ñ^]˜oÒ4Ğû÷ü”O'''pu—qÁHI?šÃu,áÎŸK®ü8JÑmCÇé…G~Ò*!gŒ•!¤Bñ¼2wN·Ûÿå|ÿ0_n:cçKòşi—ïZ¨öò^›ÆĞ9ãÏğÍåKãc-ÿÁ}ºäÁÁ°7ıŞ»¬DäÅÒÈ>tÌc@å•Ú©#sŸX¬V¾·1¨Ù_©‰‡ë4©Xµ
ªı5D*—Ğ¨àöDbàÿ>·¯£ æŞŞÚrA.ÖÊe4ñÄÊØ}s8„ıL)5ÓLí´È(ôcî¨5¥·¡[-fˆœj!ÀÅì€5âöÒPIñ`s_‚ŠaÁWø#Ë)hË£êæ"¥İ‹Ë uyKy©Á ¹…@fÁ9ˆEÖ}—¢óÿØì•i/v„Ú5C4bã“­ñËN¥KY°'Ú¶*B:Y{fãò%È‡‰ö¶séL”P˜qãcƒíbé¸÷âWøpJ†n@1±‹2
;=<Ë	g> |Âòà²µ•ŒÌ’|Æ0*í~kxyÓä¡÷ú{h÷û£¦ÇåÚ°~~~×¿ºï:Mt{ÉZ°2ÊÅ µ§=´İÒ¸ƒmèjŠ&ävÄc¬ŸÙ„e
.E¹à‘¡Û,¢DˆHœJÑ‚eÇe£‚9™qWè€gåı´áÔïËÈWL
B\ë10à„ûªM ö©–]€=1í	Ó-{W°:®`ÃÃ ğ#ÅJQ$ùøäÇ"¥(ÌÅÁ+¤Ôä¨­Á‚$—£Løö[£fË²J›?Ëu a’Û£U£\g)ü35Oëıÿ8 ¶‹îù”íä6ƒšh¢™ú›å./Pü1˜8Ş*øŠû.UÄ%$¤‚–i¡¬£íìÒ.6ñRÂ•Æ?'Ğ™!:Vh}~
	ÖT¬k¡w¾²&ş€Ó€´UñR“p1ÓF¤æ5]ö{×ö¤?ÓM©,ŸïÅäb	xlkÁ½˜Ì±(°àC!·zQGœ[Æ±²1$£™?·¤TáãÇlsÕ*8îĞµ?Üw»¢kO¾“÷P­=9¿¾M°¦ŞwÇÛ*4›PUr)ª›b³X"Šï%Ò´]wºÎW©úˆ9 ÀÆ¢e+µŸöTîåî3íÙáf…@Í<8ëQæóÂmy½„¥~<<ù‚…LÃ[Zäù³£ĞÄ›¦Ñy¬Cñ>˜ÓDq}Fª}:4Y~®®øº *ÿ*¿1† õü˜20ÁÚù÷xõ¸~ò$Ñõ–¾h*–u—YGÛÜ O^ÿ?ÒşĞL™?¨€à3+=ºG4_ÿ>?‰cˆ-nªŸÂÁ”»F¨ï³÷;È	dT¼åÑ™æ¬ìX¥ˆøğ¦vÁ‡ï+>åx”Ùõ{Ø5õ=«<zOÆI¡Çº4ÃW¨½°@¿Ü$W
z„¡'ÍA$Ö¥­F¡z7ËÄ£‚ô¼ÓãĞÜÚéNî$â¡Ø»şåê–¥­Ëg–âd¡Œ%ï„‡Ò?åL\S˜ˆ(ã––éQæÌlâ/ËÖËğà|oÏ|ìâ£J0T\vuÂf ‚ïAÍ dÍ‰aIoB¬©}8›aù3Â;Ú ¹øã;:1¨ÑúG‹xwj¬NDÂ+ÀÏºÖÔëy¯öDo¤m}pÛnÔñ‚¦_j[ÌÛ6âıû>Åçk£XS§›\öTÛ†üAÔôûi[,ÿQq9ê§|¢\YöÆš%ÕRùúêN»Óƒ§kª'Õ†õâ_uOğ›Q©ø38µÿó³?[ìßöá»ŒÁzQ«Ù&<%øêT3¨â[¯ÜÚ7ìÕË4·èìe»ü-ªA­vNÍ«­–.LÆL*}.tç¤#érš*yJ¢¾73A[t!>unBôI·ƒOèÿ6VC=ë=xœ›®tKiC"¿ ^€‡¿_¤•Bqjj
ˆ˜,À/!‘¯PTš§›˜™§dçä'¦Ä§$–$nçßÁ C Ò°ÎxœµXİoÛ6×_A8À‘…v[07ÉÚÍØN_†Âe$Úb+K)Ùõ²üï»;~H²ä¬}XbQä}ÿîx§v;­«ô›µŠ]q½yÅ+YäA0eq±)•HE®åV¸ÃşJşM§˜6¤rSfÂÓ2]—e¡*™¯Ù¦Î*	›l­x^±j_
ÍxóLT‚ÕÀãŒÅ™ò3ÚªŠ/"gó51ÁÉ	ûMğªVBãâÄió–¸.ˆëÜHB6íèxQ$Â„­’Ã.”HàWòLû­;®õ®P‰ez:º¨U–ïr°Ño·h_ ÙL¬@«”-HkÃŒT´¢n¼x%ÖRWÊè‡&£W‘glœ²ç7&•ˆ+v?»ÖÎ³pà­÷'^2FJ;ó¸(EøÀµ ¾q,´gç•*2T ÈWÒªO’Ëú!“±õ¿çOÜCdºêÓ›ÿV~VdGğn­EÅVuNŠóLV{Ø¾Úp™1À”\5üîT±’à‹FAãò†S#Ï@h- p[’xmØÌÉö3À+l¦õÕ¥ˆíqóV‰ma´b§ƒNÈø¡Î¾¼ áfŠğÈ¡ºs×
Ìl jpHLnde¤OëDV,+Ök³÷y…ì×²®Û[Ğè  
G¬3hğôàJñí„<şb´BğÕW¬j#†Œiƒrç­ÖË°S¨=¥ôôîú°ê´rû*OÊ45ï¢íŞŸ>}J«ªînçñRFÛ—QöFStÊÀÄzíÑ }7Yü€r,B,$ÆË2³^¾†»İ.\jÖ*yÅ$AÙA`ë8¹UÛ&ÁËñ@ı!
:¶Äcç°,‘i€ÿÎûO+K@Ïùc{õd4y5¨k‡"ûvrşH?–ÕcŸ¡‡J÷+vÎ7@é·yşè†Øÿ4îÖËCÊl.)‚Ag…–·––a×P?ıÿ#D«´U–~ÈÆe
ò!ĞT¨Œ-ÿ±¬…`ÊX£gT†!?ÿ;{ğ9£Mú;>kLÀÇ€Áß£:š°ÑÍM›3£3³ÛÆ †cP¬D ³‹¯ïö1\ñQÌ³ìêÆè£%m‚mû€¡€–C<-!	µR‚'lµT8âÖ½'*U‹ ‘÷à+ë%íİôöê˜—Âû2Á}èàû#”‘LÚâ/õ:\.¯Ş_-®¾ƒÑá•<<2ÌÖo€Kmt÷ç"ÍÇI!~mEÖ9Ş¥;ÓxÁ‰¥uÖÅÒïÀê½Ü(lô±0²ô¶´÷·ô<•¶;ğ>h¸Ïìö€+"jAB§.Î¨)ñ•n€†Z“}(¨O!aè£7HëıY0šİSldÎŠ<Û¿°*,B`Ø¶õš†x ö©=Ä¨{ÆÜ.(šN<Wk(O¿»Ô`R/m´D¡c%Kª=pd†yß*t2XxMqêàÇ˜1  ~Òuû²fôsİlDæfk¢ãcCï‡³{Ô‡E;ÏëÉúÁ°g-ıæÈ7Äty³ha“¶1¬Ö¦wó×éşMpIoÀpÁ´»P1Í¹sy·‡#nÜµá?°›"—Ğâ Ú8¶7U×Û4š=}¹™lz´A”Ñ°Ñ›|é3”8q„–ü„½³++´	Dcã !n|ĞUp7ŒèÛ!ª"¤ÉE’‹TÄ †h˜†¦¾"µ’ôšÛ<’?v2È<Ù`ÃXà(íÔ†·æÒà¦ )ÂMà°¿jèbç¨;ön±¸³í`—ÛÉ*uãr«CœØÁ~'±×?úLšØ'¸.•¨|éeWJŠ½s£h°HHPğ4:3O8ÜR¤™q-ÃvÉ|NFµ
ä¡¡,2Pš­Š,+vh©’¤$åºMu:iN¶ıjm-
ğ¥¨à–ƒ‘‡vÙ<YÒ%äÂ£®#LÚ^dô‹Zâšªa†á÷nööû¯Kº….4ªÄé”|	P­†Ä"2æ9{€äGL‚º[ÉÉ!h‘ŞaÁ|`òuKô’®lÑD—íø^c$Èÿst0\AIm†À®’øómEÊuŠöÀh5‡Dş[…_3Ó5e{.æ»LjOŞ~rèMêÒ	¸¸Íõ½ëÚè¼C"ğü,ÑùğK›óõÛ¥P°3H:ôÂ.˜%²UDŞ¸äGàÂEšBkc¢a?÷ÙÌÑKI…*@F„¦É_×W‡¬=÷àºÓéÓŞ@€oí]ì8-ıšúA„Æğ%Ö¬Ùğ§¢xœ340031QH,(ĞKÏg˜¸o#‡3ÃŞû*§züfYÆŞT ÇK¸xœ­VMoÜ6=K¿‚‚2ÖT{uëC³Ş¦[$A7È¡(š¢$b)R¥(ÅÃÿ½3¤öCãºu/»ü˜yïÍp†TÇÅ†×’ğ®KSÕvÖyBÓ$Öxyã3ÖÊ7Ã5¶-JµQ§w[^ªâ‹·V÷XVª~‚¡4ã¬´­kéĞFøgûø[ôª6\ã¤ã¾)*¥%pÁÆ«Vâ°ßö‚k0Kî¼Z.ƒÀÁqoù&}mO[%œ=õ²í4÷²PŒÅ„3E€ª‘\ûfÙH±y6EÄºvŸ!½rª„“úÇ4OæW×Á Èó\ùˆ¶ÉY™Êñ+±³ÏÒ<Mı¶“äç®#½wƒğ·wiZFwò3ÍÉ	îÜ¦‰“~p†|S°ØÙPròa0`,ƒµğ7"¸R“³s2Uûê–a•î–^AÖÎ¦¤y&¥¬ ŒèIA\r…8ZKw¡Ü‚ÀÌnqª¶{ô{pUy›À@i±_¸çšf« ©–-÷ÊÔÊ””ÊeàQ$EA.dÉ ¾‘¤³}¯®5 òzâmXeèƒõ‡eÆ÷a4üñ'd ‘qW­ì7«İÏ@4İËÏ$c,ƒ_@éDÍòÅ^ÆËË9©œm£.gGUÊ2ê
2Ú¯ÌHw’c9œ=@0†âåMÇMíµ ?Öƒ³5r{A–o;ìï~ŠjÂE²7aDD(˜•XP“ö³Y`ı ‚CA£ïzI¡œâ£Á‹sb”ç<%¬¾X<{¼ÔXl,@ßgdóöCÂÕlåß‘Ï8‚€VNz^¾µå (D.DE°şš¢•±HFîH/İ(]¬êĞniR[‚Íı‡>“PL£`¯]'.ƒƒ=Ì"?Ğä?ŞÎ„Åãb—CÍÍY`«hFû9Jhø3rûr¼ËB>sô?VxÓ“$ûN0Ww!3è^‰Æşê}wĞş|İˆù¿èşkPu¶|ÙpClÏ.Ãëµ ?€i|ÉØ;ëUµ¥h¾@“5¾nè`6½bìrıú÷Õ‡·ˆÚK-…—'‡‘á§St>KnFvF^Ò˜O(¥52AâØÌhş8ÜÎnBB€é¦üÚgm*K³˜S²ºQ.¨÷ÎvÒé-Ş¯Sù%vÿp<ĞûWbÿ±?ÈÉÑ]°ŞÚ†œÜkày÷½ISu=ğÕp¸ãœÎ“íözD®sò/ˆèşÇ4‡mÈêßh.@¿ë†8xœ{!¼Ft‚k~biIÆd7FG!0K?9?/-3½´(±$¿h£üNîÔ¢"[ˆ2IÎ¸É·9yŠCç@xœ[#ºAlÃa&^Ï¼ÌM-Ç‚…êÍRÌ9ŠRKJ‹ò'odÊ®OÎÏ+IÌÌK-R°²UÈt†ñô<áÌêZ.ÎÌd…Ì¼´¢D—üò<…Ô"°òdTÕ©%É%šz\œœÎ •š@™€üâ’ô¢Ôb0Ç½¨ ÌpMÎÈ2ØüRË=5&/dÕÚìÇÇ
 ®ß<•ê€xœÛ ö[|‚KbiIÆä…ŒÒ Fj^IfrbIf~~r~^ZfziQbI~ÑFyn¶Ô¢"[…É¹ò¿l¯x340031QÈÉOL‰OI,IÔKÏgøüLnÉÂ¼½'û4…œv³gí8kQW”Ÿ_Rr$Cò­±ØÜ—Ûƒ.».à›0¹]Kª¤8µ¨,¤F›7ëÊ÷’,Î"³‚„ª#ÌaÉ¹] º)NénxœËÏŸ-òå³ëêMé6QKm‹’v?P ›UÔé%xœËÏŸ-¢ñ_ú[˜ŞÊä´­ÆëCÖmäN›p ŒÖs¸¬xœµVKoÛ8>[¿‚°U¨T‹î)@µ`Ñu[£Æv-R2ŠTIÚ›¤èïõ°•¢i€f}°øøæÁ™ù†lyyÍkIÊF$‰jZë¡É,-­	ò&¤0¬šø©UØí·¬´MñI•;îÄŸÆ¶Em_Vê&ìô÷PB]«—€»åBwÁZíP[©ú@i@i[×Ò=ØZêï]ôåÎj}W´Îâ®ßrWŞÜµÕë7àùÖqÜ›øiyØ•Ò¸àö&¨Fâ°ûfIrà£©-Kø¢äœ<ªØÂ67âK2›ıåå™üR¹ “æ Øì /Hº A Q&Xv’ø[dVÖÔS•? •u°¦‚âZİñ ¬!°¤ÊÔ¤İ;ˆ„õ®¥ó
„LX;ùioÎÈp¶nø\LmChV= _12ÕŞ”duŒÍÈ‹I\ÆI¨+CNÂ—|íE§f)”ï=ùœpW{ò÷¿>88F†ú®rRr­¥[*—˜ÙkrvNúÌ±EÜ£¯²d¦*ò619ov	–4M/œƒÀÔÒHÇcl J‰P.ÍğTÉ¬(ÈRVÊÈ]ˆšW[¨Oú˜3”ÁºIf0ZÇ-ğağ-UÅŞYeè8§éè~–“”±şA‹/Pkšå£1Ùs¤r¶éürö „_Ñ†øs ƒKŒ±Œ€¨Lct^Ş´˜–X7Z•àGföAş·ˆ#
yu”D%¿v¬Ğ´HoßŸª×‹ÆVqD†ĞÙd¬Œ“b{LYÈ˜Sğ¯­sœ ×C·èfd‰¤ }Odsè™µ³{U›“ç#º‹õXŠèC4ç^v»l= )5Pû,bò®Q¸Ÿaõrïª0ˆ]Î?ğ{ĞØ»‘×ï­ øèƒ¹rYIGÚšm>ë›åœ-4tÚñcşìœ¥#İzÎÃjWÌÿ/gŸˆƒb[—`á¬†óß6:Vî/(ò¥mŸF¶ò+^–pÃ]{-ÍÓi-µ‚Ëàéôí¡zÚ° àNœ<^ÑHË-RÒS-ÍØ´¢ıS\ÚÈ°”¾t*Îh¼±ÃĞ>Ò‰î,4 (5ú àDùÛ°‚îC2Ãvˆw+mÛ÷CÇ<¶Æ¢ÃJGqDÀk‹mZ(ÃPÑtí,&MáeşÜÀdŒ'râ(ƒ:Æ±Î­¤'
qùu~<,ş´ø1lù	1—sHÊøXBñï:¡hLQ´ÿVú%"/7ZÊ–şşŠ¼ qş^i­¼„!:âM\N/áıáwR IŒÀ !Yÿ˜a+^ù˜·ø*ÊÆ×¸½ÿ¤ş€Ë·VxœµTİkÛ0¶şŠ›ŠÁ¢ì-Ğ‡-m(£la¡{2¸Š-»Zm+HJY©ó¿O’?â8ÛÓ˜±¥ûøİït¾C{½ğR@VçÉz¯´‚"\©»MŒÜVJû|ØÅ™ª™‘ú°7¢a¡Ï¼ûâú#ËÔNsŒ(B¯\ƒVÊ®ênà*8â•ªkŞäï(z4b	¸æ²ÁmŸ]ö%<¡Òî“0y[šÎ$¸PD:’N  *Òï4	9Ò”Íà)Ğ1Em°N ĞN¸œú·ÒY¦p íç¹Ú!®×¡X§uïÌß‘z=y;®9ÁTJƒFüÏ\İÿ¤3®Ä—;Ã^œõò\I_õ„‹„3´£pÆECÚàñş¾4–xWâN‘q®}Ké(L¹XØ’6él@±àJ&
»7¸•/î¹~ã¹D>ôi‡&ƒ»_";XáZéF´óx+ìZéš[+4¹ê_¶ß¾Ö÷£›÷¨öøS÷cN¶B¿
g#ÔMc'5Şm¤±¢±ëŠ—ÆY¶VË¦üÁõ†4²Z ÎTSÈ{É/ş[<~ ‹
Y‰%c2şiTƒ]¢Išü³RÕ†`«Ê²Éº¥à•N¾ÕjaŒ¿
¥¡‡ı¥¨Åó[nù©®3Ë-í<Ó?W'´†åÍpMÅc÷Q$ğÎ7àNì‡ÁOC¼v©«ª!øNk¥—¸ËËãâ ‡¿)ãx¤%×N9ºÁúåŞ…¼)xRËjÃ0<[_±õ¡Ø%±)½r¡…B)¡¡—–6¶,‹X’YÉ)mÈ¿wí4PçP¢‹3³šÑªÅb‹JBaJ!´iHDW&Ä<)ên“Îä¥Şêiô…¥Î•›]›iÚƒÌ±mÿ|çŸ7N)IgDßV·wyá6„±H…Ø!õ¼¤\šæp=€ÙÒƒ¶Ü‹(zõr£üxÂàºæ#8^Àbõ•#h»M£}Ø•ÚAp°Åj‹ƒîÉY5’Áeº•$¯}6¬H¾tvƒ—ã¦7Äg÷ãÂÔÙuŸÑCŸ¹êlÃ	'NR¸%L2td•™#¿¢?W%ÜÅ3å”‡÷H[•Âş¤;9¸D$‰øåØ/`6nvö,?“4ãdI*"]A]ÍÁê¦'Fü²_›0ÄœèØüìÛlİ)$–?`Àægc§\„óı úÎ™ïbxœ›ÃúŒuBÆDs!¶äü¼´Ìô>ËÅ¸8!=ŸüÄg0SC“kò'ÆU º­à4xœ{Æú–uC‹g^fÉäv¦x 8 -¥xœ340031QHÎÏKËL×KÏgˆğãd[îùåî•ÜE=k,æ°…\n ã´:±ğxœ¥W[sâ6~†_¡2Ó&édaŸÓ¦38À„`;Ûît:^a£±¼–LBwúß{|A6õ}ß¹èÓ‘œÒà™FŒ"Yó¨ÛåÛTdŠ\v;½ˆ«M¾êb;éÑf‰w2e/üèµ¦ğgşnC³=ùà_%D,@*MTkêg±@
Å’ÚbõºWİ®Ú§Œu$Dª,ùÚíXiJ“ºq–TÆ‰RM$TÆ…*Ê˜¬Õ@5Á¥Û4fö«rY¶ãkP?Ğõ35©õ@e‰Êö†µ(Íÿu»;š‘;*Y™ë¯¥¥(@^³v²+È`”'ùôYŠä¦GÓÔÇ2ø(ñ¦g-¾=ÿĞû¤sºeg€	˜sëÑFh„Csµ!ÆoŒ08,“À"Ò8.%”gTq‘‘â—4×´ :$Uü¯ıïPA)‘¥*ŸşW§rÄv,éÊJV µ2&ƒ¢
L>Ë%|#Ä&¶®{€oO<³dÆ×Lq,%¯UµİW8ÁËŠ:Ö“7ñ­áĞv]ßsì¹?›ŞÛŞ´(ò’­!ÌMÓƒá +ìßö°´ï—¶;9áÂÒ‰…ì iæ v?€	o&€Cgd›ÔEÕ«SÕ]–Ø¬øZdDmya+ú+*ßÆWµwY1ÕR­Ôƒ ]¨ìÔ™û®=\Úf¼ ZWMd
ƒ'qË› 
Ry¤¯VÄnß“-£‰$‰ 0ôÆ.U@·Ê#Ğı¾æ,ì ß+PÈb¦Pjâ™3 x¹&ìKÎw4µÆûšñ†¼¿0	ş¨J»á0…Š¢Ôi’ˆïXË†DB	Ş\Ò-}õ¡«ŸL÷ÑúË·Æv™ñŸ`Û‹œ(ZEıÍZ/vL'¨×1®	WD±8–Ú¶ÊÄl,øM•æR.IJ3Åƒ<†W’ÉÈã¤YAZzË°¬öš÷&ËŠJXÉÓfÛ‰rªUr­3±%AÌ±2È8ÌŸ¸ĞÖôµâaÈ ›xŞB‡¢*Ìòñ1’“¥AˆïÌgmku6C0˜'@Sm°6|,ÆşÄqyâ	{‰ço¹p–ù„õ>‹ÌÁÚF>¹ö²ØR¾á¬G>-×Eäè®>@N ÃÕñ!àÑBA˜P¶¤†¢Bá1D
¤EùÜz°(nI4cV“q0Ÿe›fö	Jlˆ´;M{L‰İĞÇvx–·j…mnWÆĞOÏ”LÊØß‚¹Íèº3ÿ:¬ŸÖŞáÆaèN«§í‘dÆËÅ°ê7Ô©ZŸä ıÃué;İë}ô=î5 Ü•wãFe¸·ºŠ¡yŞÚF$¬°6¸¬û®CÖİÌa@3ÙĞC•<IwQŸió	–™3öíöÜÓË<Ô}h6¹&*ºóÏp6Ğåfœ‰<®34šß$/§Eƒò.ƒKC&Éßÿ4Ë_R®
óyÆ»%Ü%–:UO¤<hë¹Q¡õç,¦CcIÍk°±¦#™´»ŠLZzî-?ú#w^°­ó$ ruy…ƒq™¾%	{)~_^Ùf‚†µ±b2«Ë8òú ¨ŞSúœR1LDÒ>İpB¡'Wø_–eäæö~‘	<Ç.{‡Hz×ä“àê7û	‚ç1ºî€m}Ù\œĞyéõ†ü¼
‡` ‹N#Çs·ƒ/kıQ¾M/»¸HåYÒš^ÕD_²á*¢·È×zşÁWŞ.úå«Éí-©^÷Ê!ÀlP„ğ‡èÀàó˜T?Ä‡À÷?½•é ï‰bxœk”[)7¡U%=³$£4I/9?W?;5§8µ2#3=£$¿<µH?5¯,9?/-3}cc »Ëã;xœ==N1…E‡-¸ \íJ–75R*ô«(2ÙÙÇÆ&B¹w ãGâœ‰ñ
ÒŒŞÌ›yó½ÃËç™P[›WMï<¡¤Pô²ÛàÇûëÉ^ÃÒz4s¤a¡!®ár
æjôªI-…ëá‚Í')„ƒ¹±ÙúJ]E‚’Í.€a#ÅÏRŠ¦ö. äÂ6¦ä<&I#ìËØ”£(«ûÑcˆv2qfyùon£Õ¡cêêÀ_kPÆ(®œ’š’ªj]8Ä¦0Ä.f™»’¶«¸9¿µ“ùW{úsôòíd–ç€vxœÛ¦ğ]`‚çÄ’rfÇ‚‚Í7°nváea tËzí$xœû.°Cpƒ“§WxHpjrQj‰—eæ¥+$dççY)e•—Äƒ%•RóÊ’óóÒ2Ó­”€Zâƒ]ƒ\C”6Ë2E± ù
w¨xœ31 …´ÌŠ’Ò¢Ôb†w™…/'õÆ‰k÷[Æ$<¿%&aV’›™^”X’™ŸWÌ ¶#: ıªMöêÿ}
Ç¬²·_ßŸ ÈÒ£x340031Q(ÊÏI-Ö«ÌÍaØuôd`>ÃÙCÖj+¯&]±{ûp»!DQqr~TÕÆwBC®Šzì5ÍöË™ÿ1çwÓl¨ª’Ôâ’øÄääÔââø’üìÔ<ˆ¹±—g;m=¦gls$}/÷ÎL¦|CÖ‘œ“™šWQ[¶®[áçËYŸo™ùÇÿ{mCÉ%=dµ¥Å©E•æG®°ª6[7/uV	“â®³5G—É  JÍZ6ë€6xœ äÿÓÓ¿ÑØº{»‡cUV…˜edÔ –­íYÜÆõù°HxİAOƒ@…ïı“Ğ¤zÀ ]ªô¤M{À*ØRÊ‘,0JÙ]bÓ_ïn‰5jL4Ñ¸‡ÍÎÌË›}Ÿ¦wGëirdäëØëé hRá­z ÍóXŞ e&»¶mà1-'Ñ‰™^š#ÑÈ¶‰œt  ÁlG7M…†îòš(ò’qÊY:æ%Vï¼Y]aüÕÖaÁ=iQnß7ğ¶A¦ÌAÙş¬ª-İ¨<¹¸>JT0rşR3¬oÑ¾iôÉ-Î®
^äO‘øso‰Ãï†“uà9ûË¬‹5¢E¾œD…™-Ü 3KRYL…´ó½Øõ‚Ùr{~tvŞ)Ú&û ¦7«Ù›â'Ø'§ÙsLëmömøÃğ?ç®¦ÿyz¹(Jö›Ä­?ˆüLDÒã>xœkà\Æ, ¬Ê\Ê
Aù9©$YŠ@´®)£ÁäûŒbosÊs)(¤e¦æ¤[Y

y‰¹©V
êÁ¥©E¥Å©E“­XÜ&ó2sÃ5‚4N.bæ†*…ª &À ğ¾Nxœí’1OÃ0…÷üŠ“2Kiqd‹Ô]RD1Fï""LÙÊÏÇØEÀ H¨bªËz÷üîôébv<qÃVpÏ5b`E§¨€ÄørLO…»tr–¥tÃÓ”Ñò¶c|œ‰ëEÎ8Ïó,ã®’.€ªñ<)‚¦Y¯ 5h>Õ¥õ)4!×k.úÍxÉH=Lv÷N/•&üR’1¾A‰HøÙBÃ¨¦EêÅ¬lVÏäEé,a+œ˜lêv]o«û][o..CßyÂ/ænUîªwÇIà\}§=èÁÒ¯ˆ‚íPİh´ö!ôÜz¡ÌiÀÅçµ;¯İÿ¯İ+“xnD¸LxœÍ‘±nƒ0†wâ$O–²	°Uj‡İšs+#Û¨yü“¥j#¥j<Üp÷û;Ë_J¿Ÿ4IáI)tŞÍ	'¡ó#“Pğ²±"c¸ñ1L€ùT‡
 »0‚aÉ£˜W-åYÇ©<då¼(„àaÂHáå"Ïóˆp<¾>Co,øÁ¡2S‘˜½Æ±s]'ßÜ»¤ŒKŞ¤Wƒ>®[¬9_9.2‡ön¢$ë•_™+g#Æ—Gç·ÌhãØ&1á”™×o´(;Xı´Úã6ÄË¬-ºFúÈYUQ–QVã55+ÈCE·EøAÛzhÿìaÇ
·ôf±~Ø§õMş_Ğ»P÷½+xœÕAo‚0†ïşŠ/Á„í VP¹-ˆ;¨cà²Aú1‰(ØVÿ~İ“í°d‡]ÖCÓ|ï“çk^Ãº£g@ØÔ¸“ÔûË¼g,V`–0{ İ&P7@ÍÔÜó(!ºã•EF­bèø¥¾ïyT%dd½Û®AÈ²xUËA®ªš	Ú­œUÚ¾Á³ÒK2×yî˜ŸÀ’£TYß-úé‡Y{w–•Ó%Ã0ô_½y•û8}È–4NöÇ{/ñ§dYÅéb$ÜtÏmŠZÅ‘ÕK™x­„k);§ÓÉFı]»l·šUK‰,/>VÏgy<{Š’4ŸÍŸ¯®5qèØ7"[LnÓèBüM—ãŸ»T½´;öÛ2İ]æ;´¹ÈêˆnxœkàœÀ9á0£ÁdG&ñ‰·]ÜÜœ“]²Ò}²<ÂŠ½üsJÂ<|-S+Âõª
"ÂÂC‚<ÂŠŠ},RCsJ3œƒ+Õ”JR‹Kâ‹‹Ëó‹R&[³¸Næev™|q‹)YFN¶bqšğƒ|İ 'ëJö¢=x340031Q0202244“&Æñ¹™éE‰%©ñy‰¹©z)ùåyzÅ…9Ïæ>š½éâ5gïnÍuåQ7=é	hHXiX÷Á;>'‚ÄErH(¿Óaºî÷d#„nK ÌŒÍãK‹S‹ŠÖ‘O©U|™«PûÀ÷?:ÿoNO#Ô¾ï/¢d¸¹N²8-ãfw+íc][j#’6 ÓØÀÜÈÂÄ">9'35¯ÉFİC³wXL™”ÊÆwj‡¯]Ñµ·ñk…Ú¹Âğİz·Kvá|Z«ùlKß*å˜>/NÎ/HE²RôCà±xÇé7½{'ù0Gpëê­ñê„ÚøÕ|İ]	_ÃïÑ6­¡Ìgdrï¾VïÂ¢ÏÈ(¾(?ÙBşÉlwt¿ïš  ³—wwÜ×³Egtği„Ú§ÿÙşà/Ş›µSïnQö½P'ğŞÏå&m&ñE©iE©Åñ%ùÙ©yH>µú?Á³"&Ã@ñîUÏÿ…‡¥DŸeÔË&¥š$·ö˜<p.õ6Ÿ}g† ı¦Æñ‰ÉÉ©ÅÅ8§æ`0Ñ•ı¹úVãÛ_>(0½úŸ öÛVÏ˜™¼Ô÷‹ãKvsİ(×mìf1ÚLãKK2ò‹2«K2óóâ“óSÃÊëÿ*<Ø½Ùº^]xûZèÖÉÄuËq“¥¦‚‘¿²/|8¬n»yKƒV; ÍB[“áƒ(xœA ¾ÿÒÒ°²ûOËŸÆ{ŒÁÃŒ~™u!“$“êHÌéN˜¿YlÔì‹g%@t¡tü§I¾“*“pb²œï0xœ    ·
xœ]±
Â0E÷|Å#Sº¹uŠõ!“‚[	ÍCš–$?_[7Ç{îÓ^QX¼YÔFödº·_ 5À×5øãœóÂÆXû³­8+—J˜”¡b <x›ìÄ ì>Æ;EJ®Ğø:AU61º'q(ô.{HJíÜSRXJ˜ãßÍêæÉ¤0Î²xœs	òPqtòqU(-N-*¶æ 1¶U¶(xœ¥’KOƒ@…÷üŠV´‰1­»Bœ¦a¨0¨uCÆÎÔ’  Sªş{yõãÂÄÍ$wæÜ/÷œ;nHN@8aè,àÍx³®S9.´.Í™a¸½˜;7A­U¥a ÌTšh•¸%s'öxW$¯*W•Ø©d?±l,Cê;á
wd5êšZ@.Ş”‰'tNh]N'6bFïcÒÁbÏëµ¥Ğú£¨äI{uaÿĞTEvÆš¿ã1Ü"×©T‹}¤´¢RØ¤ŸJ!I*ç³õ8Ú1ô¶¨3‰…:OßkÕ‘Ö•j¼ËDìLpê“ˆ;ş”/ºÏ#Ç¼Ü8	ãÉQ8DTÊ3¤ÊÔŸ­K£17`Êøà)9¬+i]¶d¶86`wéå.-r‘]·A‹î³`-rlÅ^¡È³¯æP]€†=3¾w»_ìTxœkcÅ2áüdFIN.¥ä¢ÔÉ'Ïc| vCÕ´xœs	òPqtòqUHÎÉLÍ+)¶æ =Ú±.xœ­’ÁnÛ0DïşŠN6`i E{R!bË©LµI/+n"*¥R”ı}ÖJ";è¥‡òÆÅìãpv“LÄJ@Ü*‘nä:…¼DºV\µAÔuÖ,ê¶m¢Ï“Iò"Vñ—¥@YYr¡Åt‚—Ya¯Ç…¸Œó¥.Å–y¨ØMg¸Éä*Îîp-îæc£nlñH}„oq–\ÅÙôôül6ØHóåy*¿æb,ÈÈé_djè]ÍoyjÉï¸Â€?µ7#´¥ÒS80?œ~<0ç8:Œ•ÅºeLEnp_{0¡ó6ô#Ò“±ÊPpùÈìÉÉ_`F&µk­!mŒu[h”T>¢äzğÚº {Gdè`zï")4Wr%6*^İà»TWÃ?Ö©ãMò,©*Fá!Ñ®1ÿ…c¨¢æìÿ?tÎxW8ulítõ	Éğ-hë=¡v8ÎrD_wh*Vğdwä{üìaÃÛÎÉôBÜÂš§¢ãy·Å»v^Û×mœ¾ŸyOUØ	j‚
xœ{Èz—uBëÆÎ& r³xœs	òPqtòqU(NÎ/H-¶æ 7™°°Cxœ¥“ÁNã0†ïyŠQN­D%ÄÂe9eS#¬mS”:,ì%2ñĞZ¤v6¶QÇi
H°"ÇöÌçßãÒœ$Œ ¹c$[ÒEô
²ótÉ–;'ÅDÓÄ—Q”öÁ,ù5#À«
)­~Be`ÿb)bèR`J®’bÆÂ¤\¡Â–[,ŸÏGc¸Éé<Éïá7¹?é³ªZ¢²e—\tdÅlv0™@bŒÛHµ‚CH<„¾Îá6ÉÓë$ß@¨ªZäÔ¨Vvº… ½G8ƒí‡àXÃ.î­‚nù[ç‡ßğãÿ	¸md‹¦ä¾ŒÎÉ’%óøCÙu˜ÂßEFÏ2`I—Æ­Ô
¬Ü ±|Óô@SéIï¥Z)°ùví+bÖÚÕœ’ÿî^ÇË¶(¾&,-òœd¬<î,âñ}ˆÀ¿éî½ëCínÑ„’7ØúaÃU…û† Ù”ÜÛò¨)ÊÁ°¾±¶F¯<ïÏø„³7İ{ÊŞ¶Ÿ3Âğ¡7Şø2¼è¢éŒÀëŸ »ƒZë'×˜èÕTF2ç‚%xœÛÀq†e‚kqr~AêD7{0CI!Ì1ÈÙÃ1HÃÈÀ@SÁÏ?DÁ/ÔÇG!ÔÏ30ÔU‡K”RR‹“‹2J2ó'÷0Jj@3‹ãSRÓKsJp˜¡39–éd½?H[^b•‚sQjbIªBbBf^Jj…B~‚:Âä<u…Ì4…ÊüR…‚ ’’|…âÔÄ¢ä…¤J…Ì…´¢ÔÂÒÔ¼’œJ.ç WÇWO?×…Ì”Šx°G@®›¥àï§ Ô@r}’¦5 |‹U¢j€Sxœ;Ãr‚eÂáÇ[Õî²xœs	òPqtòqU(ÊÏI-¶æ 1xHáxœ;Ãr˜e‚GQ~Nj±‚×DG¥¼ÄÜT%…0Ç gÇ #M?ÿ¿P…P?ÏÀPW]]…Ğ¼ÌÂÒT^.¥ä¢ÔÉŒ6Ò`!™¹©Å%‰¹
ùi
@™Ä’Ìü<®ÉVŒÎr˜ò9‰Å%
¥)‰%©\“ëM¥0•¤¤æ¤‚Œ˜¼…Qdåä§ŒV²`ÇÇƒ¸
ş~`çk@<¡iÍ ÙNFºxœs	òPqtòqU(JM+J-Îˆ/ÉÏNÍ³æ n‘’ì†xœÛÀ±™}‚{QjZQjqÆD‡ÖÉŒö›;3*CÅâKò³SóŠã“s2SóJâ3Süı r“˜ÕÑÔ•§¡©ÚÆ¬*¦
L!«ùÁl wS5á[xœÛÌ¾š}#KˆkDÈf	ÆÙL /Üºxœs	òPqtòqUHLNN-./ÉÏNÍ+¶æ m~ˆí‡xœÛÀ±‚}ÂQŞÒâÔ¢øÌ%…ĞPO—É’Œz›;W0  ¤	ïá*xœ[Á¾€};#KˆkDÈfaÆ©L -X¶°xœs	òPqtòqUH,-ÉÈ/Ê¬J,ÉÌÏ‹OÎOI-¶æ ¯n!çˆxœÛÀÑÊ1ÁZ´´$#¿(³*±$3?/>9?%µXAƒk¢·ÅÄû‰Gœ&³3Ú±*•§M<mÍª’ŸÜËh­¦T”š’Y”š\_Z”©¤æäìá¤ad` ©àç¢àêã£Ã5ùcğd)&÷ÉIL.@S
R'dÒœ<Ilò¦#L«ã“s2SóJâ3Süı1L.cV²Ã¢ä@œšŠ5” òJšÖ\ øsXQç€5xœkåhfŸp‚¥´8µhb¿èdeFÕÍŒµL eïÿ¡	xœ340031QHN,.NÌK)JÔ«ÌÍa8ìo”WÌ•ÊıŞWw6§•[xqqïCˆÂ”üäìÔ"İäüÜ‚üâT½T£TİœüäÄ½ÊD Æˆ/;ê·ñ·Yµ˜ŞÚêc—h6õt#v`å.U÷Â­“s¾{¿ØXáXax§ñ. ªµ7»xœmÍªÂ0F÷yŠÁ»p%•«.Š½Û«´ˆKÛAÍDfbÀ·W+h7ar¾3?‰D]`ãÙà§=… éIÎ$p&êŒQ’äZRk ZTEî çñH6£=FÒ8dmàˆIöŒ>—&¯dğÎA¢>ÇL`´˜ÎíãŒ89	ì‰ã[Z•MSşWu¹ß6õ;ïøK¸¹W»u]eB
ıÅS¶ï}V‡m‘PŠŞŠ7&ëù°o$åf¸hxœUMs›0½ûW0ôCë`7ãèF°’q8qzÑ( ¸¢vëşú
ˆ18x&>Hb?Ş>íj×{*$ãĞô›ë©>ìy²K©M+¨,p¾Á1)Hóıó-¥9XÓbÁöT -áI”dKŞ¶ä¢~ ©Ø³¨‘sYlD}Ö´ˆga™ÏHJ¦‚WAyeÅR²)•GWÓ¼6o‡$É•EeĞºCùjzç*Æ#a¯Æáwb”
½²WßĞHò‡d%Ë¹(Zpß'7 –q%¡Ù	¥4+&
ÂGx@_!Å)ËÎUÏÈŸ]PÏî¶áÃ”E‚šæ	)è@Y5 íd¼²B¥Œ¶·96§&(—JôK]¿Îs×«çB–ã gl¹È}Y¢U€ô8wÕ›)ÄêƒcÉ{Uš>6w£»	PËøŒÌÉ¡‡ÈÂzXXøŞGèã¹JÙQ8ó „.ôÅmî†Ğw-—q¾ÁõIR
ô¯5{‚~8àìL}ÉXÕ³.ù÷€şDŠ)ôY¹.´ÃV²O•8§h¯üyø‚=…ÈF^Z^›ŒçXåyˆ4¢.ƒÊå˜±&€k-aOïÔ¹Ajì•éÊ.°ığˆ­Uˆ°íC+„8DŞÜ0t­{6o£´ÿrîÖÊïUìX•«~Ÿşò|Úıs…ø09ªY!v™óhKÅµTèÖƒN¡e±Äj6§ŞªÛ$<ï4
•,f$‹e¿DÒx7êN#=ãzF \>Ó1ÍÕ[/ùİìÇÓ#/,_üpJóv©©ıïdZîRšÓÑBÖ`2*hló4gj$©¿Jüu%iP–mf4Şå	‹ŞuW£Ÿ½ô‹	ªûı?Ü(Ü§à„!xœëà}Î£V–ZTœ™Ÿg¥ n¬g¡ÎÅU–ŸSš›ZlÅ¥ PŸ’X’bN4(˜¸ÔQ}âkFõÉ<ŒÚlFV¦&ÆF“ù˜$&‡2NŸü–1€M½¤¨4U}²3“£Ñæ:¦UŒ¬ê•©Åê“uYtØ!r\“SX&Ï`±Ş|e## «ä&V¯x340031QHLÉÍÌÓË())`¸²aæ}›pî½Y‡¾ãûœ¢bdb 
‰™ºÅ©É™i™É‰%™ùy‹ƒÎ,î˜Ç|+áªSvnï„ı7BLLIÒËMaXÜ•"ôî»Şóíz=q²¢ğ©(LAjAN~enj^	Haùêºe;Ÿs½Ù¾yÁô¿Õ7¦ïXWXœ™R°H¶í°÷´¹İì‚õ…çzöLu4Ö†+*Ë/()zôaÓü´ã¯/©„üLe÷ò£cşò1TQj^HÅ¡9û%Âß˜Šışpìßöß}[n@U¤åä—ë¦d&¦%æ‚”~çÿjÃi!6ïÿ:¹ì-ºÊGŸòŞ„*ÍO,-É€Yà³ŒIwïç©*mú4õç‡;ŠIË{ AV”ZXšY”
ò%ƒA{m„GqNÒç>?¯˜-/—8V q+¢¦ïZxœûÏ´‹yB¤Ir~nn~n~biI†‘nqjQYj‘nZjbIiQª^n
Ã“;gŠúkŠMUü#_=c¾µfú¯AºŒ /Dz¶­xœVËnë6İë+( ĞmVÍÃ÷ö‘ †Ü.Š@˜H´ÍX&U>|ëı÷Îğ![\4‹Pâœ‡3sÎ8ÏsöŞù–Kv×.-Ç•+2Ø‹qs¾Ær`· ]6{Z<³ZQì~,8».*o6Ù’&Ïû–_3hÛFT`…’Å»Q2ûÈş$lùèš÷ìæ ]«æµĞ¼²¥ÓÂ ìÑÚÚÖ\xd{Åÿ‚mÛğ«Jm‹
šæªÍè5º®4H[Z<<8RlJ‹¿ı÷ËJÕ|tI,57ëÒª—¯©Të£Òjö]ËSD•’KQã½4ˆ°ÚñìŸóƒ‰ä²†ì$»‡¥…”ªìëô\’¼ãK[ƒå	<{9.>ÂC)êxâÏĞ:“üî§Óçéÿu=TıÅ`MÇş¿âG%÷ÖÏw¸mR%é%Uó]­åU­øOG%JlÁ˜ïJ×„3¼rš—İÖe¿)~ÅÏ°{Õ%_«&V“½¦ÌÇÎ´ZŠ†ûh{iôqm0gñj†[6‹\«Ğ„˜¤Ğ‚×7®ÅRlÀ6İ‚h†üv„ÙO¸·æú¦Ş
yÑëŠIÄıA‹¯ÙOÊ<99tAİÌÆaé•˜İ:°Ñ~Îæ”®DŒ2&4U§æ¦Ò¢õüEÈœ¸Cq±ˆcB.•Ş‚§ÒE¢øxO?®r€~fI€~=åHp¤È¿CşIØ8,ıüÅèo*+v< ÍÀ-l0ÄÆÛá[Àå;`±û–Òä	“
UÅ	rUî q]öÃé]¹Ò‚»„ßºfÓÂ°±“ÖmB­í¤ÚÇ‚j\üGœoø¹I?ØØ‹AzLâ™Dˆl‡—NÓ<˜bO™d	&¡˜=×µë™e>è	õ3Íƒ õ¯'İyÃP}»cU9ì‘åè€ºº¾ËÙ‚4QØ=û=*),-¹ÂDÜ¸ZXö V½ €v'Z¥ö"Š<ˆ­°èÅ¬ŠX#lÒ,úÅ^`Ã =‡\L2¨P\
OñV÷ØKo`ˆºk”E¶­-×Ãay/clB3t/¶-NĞ(¨´dËò8ÉÎ`n0 –¦j~Üù}—`a‘‘yoş÷‘ÑÔA?ÿ€8ùò±y{–‡ëãv8àõ0Š[¼ZÙaç¼¡;mAî'VMhWc5~ ¹yr¥Nêá™ÜYi¹¡)©cNÂ§ö²:L­/Â:Í~ã€‡£ÑWÌ×ë&h¢;1H_¾<Ë{m—åA+§²Ò{?²<MZd‡Y{Ì/3öûÙÖàO¿q÷4_m–bå4·9ÿÓqcÙ7hD÷:"¬¼[lÙ#Õø;–§­xœ340031QH,-ÉÈ/Ê¬J,ÉÌÏ‹OÎOIÕËMa˜ñÜøîw#¯˜éºŸ¶ıÑíõ~mÑ™WR”_\š\Røèª‡}Lƒvÿ:aÏOóşÿÜö¹s®9Ta>Èèøä¢Ô”Ô¼’ÌÄœbúówL#üÖó²•ÿ”À»æåe;!7¨ú‚Äââòü¢²€`¦>'›ÒúJgôÎÉfHJOÉ„*+JM+J-Îˆ/ÉÏNÍ©İ—Ô?y¶ëâú=[$¶ÍÚT¼ =Vı¹xœE=OÄ0†÷ş
”KnóÑ¬wÌ‘›8P!è)Iètÿ$=‰Ñı¼¶/ğ”I<Ü]:±%vñ¥Z#O
x8Ì z¯€ÆŞ€RÆh­Jgqß	rS²yıà¯êÏ~õN”G“’pFÒ.H9‡êñÏy‰œìR-ik!6ÿ¹&=1Eu6¹uG‘ÉÛï¸d®8r(	ïÿ»göÎù~Åh€¦àÊÌ<HÅS¢»õ³ÜLo-ó¸µj [ıN§×£}ynË3å-Ù[oh‡î¬¦›İ]ÿ  ‘^gë€jxœÊÁÂ0P©G¦@G,9&1,'”Ô?$8´ƒĞ±º#´ïüÖa~w«á|u1M¯ùûÆ'ÜÁ]]$3µ$Féâ¬¡[Vè•E8.óãÔcwm¥ Yl•¼ë¾8;z\şÏÉ‚´xœUÛnÛF}çWä‡Æ‚.”-¶€<ÈMš:@í v‹´0@-É¡µîj—İ]ZQ‚ş{g–¤L‡vĞ[šË™Ù9gFI’DÑÁÁ,?^À;—FjEÃáÇ«ë›áVÓ‡ÙÔˆÊ¯§ä°Æ•˜ùU%mŞ'ü§BçáW9ZEc —”a¬ü*¼4z8\Àê\8™Aşş¯ù/g÷é{¥Rı©øí¦Œÿú|>¾TPùòÍ›UDÀ8oÑeV–Œ± À×ó1êÌä˜C¦$j™ÅœşK¡ÆBÊpÃlÍBıIÓÔÏF{²ov%†DYª&fzïŒ~¡ò…Î9
*ØæÅ¥Ø)#r¤†×W—ÜÀFøÉ3ã97ù.Šn:é)Y`Sñ'ü „¦÷Q³‚ì\²0J™­ÔwPHT¹[DÑjµŠ¾Q»oşF=XÀÀyKƒÑŞ˜xzg²&Ş:îèßÚL$ÄÍ¯jï(t'i¤‡àf¯˜G8šÄ"éoİm«Ì']¤ÛPò6Ôì¢š0H¡u	ì‘šÊ‡·q˜¢şŠ¸*İHOØĞÇb¯pr7sY†Î%!a¶²XXtëÆtø”Wí°% ş[©Tø#(­y9O;G¢€„Õé–Ñrğ …ø] J8g2)¸c©k” :.~ ×Uèµ¨T§‘=}‚Şõ€D·î\fJ6,­l­ôX“Z+>‘9;I>i,³Ú_9´ZlB.¯v£°hk~)É4{=ÏÎNâ8é%­qvzÚuZôÄT¯˜¨B›]"JY›¤slZ{_.¦Se2¡ÖÆùÅ<ã:âŞKHOñ8Íâ³qVœ‰ñ<ÎNÆâDäc<)fGôõ¨˜VpWÂõ Ym©1
…>ì.¬ì
‹65«¬¥‘)".äµòC”l#ÿ`u]~·’öaØf×<Ü5=ˆæLÉp¤h{mu¡å®—ß:^nbÄl.Yªğ™…ìöw®İ«Z¼>5
	„³i	ñm›RÈ%mP:x¹	kô»–_ K“­÷§°†¡¾Ÿá,ºvÛµÌ:ï­ Şœ«Â:½ŒIŠ|3EŠÄn8¶Ú„ƒË÷£¤Y~X‚ôİ×D•ŞÓ1z2¾ÑT	Åª
„ÁÅ[¾I-w-­FÛ×ü{FNRJö”—ıüœëgòˆì³á´NOÂ—PiI?;?áşZÀ;kÉ÷x«şÏ=Y2qá€‡cÙ–øøœ×}7€~œ„¯\Ò8È¾·…ûB*¤ÆœC‘»cë2œÕ¦*—,ÒyøĞ4¾xœ-»Â †wÂ0KÒ"½à¦«ƒCëLNáhšFÛ Ówh·“ï¿ò#Ô€zÜ…´Fç”|B¹È;!¡d]Ç‘	¸—LU0)dyÍ«Zjº'?SoÑ©>¦e––J”ÿN›Îmô:=®È"õ¶½GJ– <Ã4<’ÔÌé‘h×£YQ{joº^R‡?;µi<í­,Zİ–&ËİAxºxœ«æRÊM-.NLOU²RP
JM+J-ÎP(ÉÏNÍSÈË/QHË/ÍKQÒáRJÎO+110100 ‰—$–”ÇC%€âp1:Í©EEùEø,àªå *Èº,xœ•‘OOÂ@Åïû)&õ¢	Bøã¥1FT š¨õ:…íNİİŠ‘ğİi¡Ñƒ&Ş:ïußìïí§·0FJ‡pMyNVuÛ@ºë^Çá;½¢êµ¡³WJÎØ”T¿3\ĞÁÈ&à˜OÔÚI¬‹…š>>=C#ª­Š0×&‹bˆ$é?t^dØ^RµTThï7äñßİ^_,«sùÖnEÓIb‚!«³yj0K<ÿ°İ©R3ôY±¬”MsS…rØàLæè½^U‰5†Ó’¾\.ÙJË,âAÒéş.Œì1À8£MÍ×é°ôV¢àÄÚ§äVNsË}«‰Ş˜°†À¥Û¦;–$U®$UÖ¼¥!ÜÌ¿È³ŒÒßeÀ$R“!ÜkËø9ÚĞ€Lx}QÛj2âÕ‡aÈ/OÎ|VÅp…Úñ£Ÿëª©zÿE…ğR$:`2}ùoÈV4/]§%ğ sKœ_ß›	¾ ‘¹ê2»¼xœ­Xïo¹ı|ûWÌÁ|´®ıf\‹º¶›úp±SK)PFBíR+6ÜåÉ•¼ÿ}ßûKŠ|iÚ	,-—Ã™73ouF7Â‹µp2I~ú>Mi¹ºz\Qn2o2*d%­ğ2§ÕÃ5ÕZâ=ú,eM™)KYyÚJ+ÉZ›=‰›:ÇJÓ?E‹7÷ç+º½¹[ÑêowKZŞ^¯îîgtw¿\İ^İĞãmúøá¾?sõ@Şß\­nƒ…$¥§wª€ÊTîù‡³rør‘aõ®r§‡gXW“¯İœ($–ş{Áßç+—ÿëøÅıª¿n1ı±{çÚT•Ì¼ª
Şoå€#vdÃbêMŠÅ4ï/:”oïoş_'«­rT[ó/I“ÆğH8R—U)lKì9o¬œ‘¨rŠĞJÚK¥¨DÁõÎÒˆõ<IÎÎhLDwjfrIVA›-ŸŞ§8l½¯İåbQ(¿mÖs„³(Œ@%–‹îï»)Nœ‹¨Óc kG­i¢›2xPòVdŸÉlÆí.ÛÊRP¶ÅQ@CU¼ÏÒ[ X×Ze“ÎhšlDe¨Kwç„¤L«>+…á4<M‚ŠFåßëb­ÍzQ
ç¥]de><D+¼»—ùEçY(Û$ù'bÎD°å:Û.Î×Ã ŸœôañıÛ!Yí”5U¨°J¬uDïDKÊ3.¼eˆ3Z»L’OŸ>Çm"_jc}gó}› È'X°Ï—Oµpnolşüç§-Vù	6</úT<³­¾VÃZòá›F“Ú„dVRÿméC*íP'¯J¤å¦:÷´‰;®­%™&FƒzL @dÛ°wršYÉ!
œ·«kö:¦…R|–¥eÒa”F;—tF+a`Î}ß+ŞzWÏ“ïHã±©|ñz¡4W–òõbì+úİ÷Wïn/&8í‚9í0±3}ŞíÀ2ÚŸ{p
/T¢D7 ½(ëçOÃûyíyŞÔsv‹ÑıÍ÷r³¯øÍí·
À‚ß¼èÜ`’Á>/¹Ø\ „»¬eDØ°´üyÍ]:Ö[€ÕT3Oî¤nc¢ú:#ûö$5õ%Ñ+)jêó~w2T~Z@vˆçÀØ„Å¸ ëvy2!¬!$nş_ÊŠñ}­¨xmôù»ÿÌçÑeŞ>8[Só!Ï„ÛÊÉ_Ye‘'C]uFş‹0`şÕ0°v"ŒàÑ	¿£ËoŞÜ/ß¼¹¤nÚDrRK‹/¨9©&µänéXàËHœ	Ád˜ÜF{fÁŞ5åcé…ª%ö¹O­8DøÊÕª6*£´Ÿ~;F %{•@Ââ7è˜ì°ßÿqû¸„Âš²
şuÌ‚gîí©´©râ–ùA“‚ÆêÛ"^m4«¹S¦qXg0AñŒçü Ÿãà‰^÷Éu[Óè¼›yd³.o³¨fÂ$™¦_hÌÒ…@D¬¦éŸÓ;d\ÅlÅ™•]jj~9¨ƒP8k	gøä ë&5õÏ¨«’d¢±‹—Úì¥¾ «–Uê`Q™5«²ÔÊ¨ÄdL¹ Œ²&Ã¼.XK5¾ãúÙ­´òíŒ6	ƒ5ëÆùJ:5 À»ğ2“A*u^›J·C Æx|©Ã{¹Qip
¥Æ¡Ø”ĞÑ6´–n½ÊÜ±tZš’UjÛ{åÈ)&U¦›\^²€_6uAdöƒ=xÁSQÄ—æ•3ªšRZÖ9Ñy¶™»º¾»aX9‘uã"(Y…|GMQ9¬«¬æTåeai/lz«Š>‹ å¬­3™sÑŸ’ûá (Ê¹®Ğ9š½Â‰È)I›m´‹©ºéd#Š¤—Ò‡§³–I}PRX>€Óï8½ì{._(S…$Ş†øÜã7œl%b’;¡ÎúyùpŞâ9HÉ‰àbÉuÖGÚÙ|AÎ dŞ—m²P¯hlf><²¾ïğ±rf‘d˜â{qØ+òiSî@¡³ĞÕjm¡u¹;Å_B"s¦vÿYƒ™@S­›Ú:0–c$47¸‚/ñîaga¬j#bÍÂ­µªâÑ‹Äó³‡rÕ¢¿p;’Ì¡¹ŠCî7&ûŒ(®¢AP<*Ğ\Ã„Íp ^âf‘$?Ny,Pm·™!8²Ã‘v‘Âc8’ßÏé$;Ó,îœ·¥TfØÔ_,9Oş š‰;ixÎ‡ã…P¡Æ…†ŠÍÛmĞ'ÂùfJql¶“šØ´n‡Aù7\\c‚lÃ´<VØ_9ÎæÒ0àWâ–5Z î9%¾øUpÊ!6ıáîxı§„$y”µ@µlùÊnû;W¤¢²bĞı¹tŠi‹w°~ŸÑáaG¸ ¨£28¬D¾;E÷zß&h†BÑ‡j™L‰ÓÈÄPgÛâIÄ(™Ü-c¤—c ¨Ğß8½ÇÚ>&.¤áò48¦HHxlÁxû=ö˜K,;H†ò¶r‹û
Rá&Èn´/İóÀ«oM„NƒÁ+”sÏš/«§¼µrÃêÅ²òÑğ*:†fï`Øï÷óñ«¹±ÅïºÅ<ù7­ü½vxœ­TİoÛ6×_qCÖ ‘moA1 ˆÍèæd¶º‡DSg‰5ÅãHÊ©û×÷xrRµH0 è‹HİÇï¾~ÇX ·têÑ¥¢xıCYÂ¶ºÙTĞN¤¡E‡A%l º»oQE„¢M}v‚B"PÖÒ¨¯ƒoØÊò×qq·ş±‚åbUAõûjÛåmµº[_Áj½­–7Ø,ËÍÛõcÌêŞŞ/nª¥ %ü{8Èƒ‰&a|÷êÂOÿ/³ÅÓàYó)’¿1˜½Ñ*r¬8N~Eÿ§2.¡SN#«ûÏ—ç>,×‹ïÕ…¢êL„v0Âƒ±”=À‰H] ¡íøDˆ	}Ì ŒDd¿‘U®…Şè@ÃÑhå½=WC4¬^>`€[ê=[ÍŠââ¾è™tq´z÷ªKÉÇëùœ‹‹³F„3®c®5çÆÅÄeÌ/?û<"¿ì«Gƒ‰sÎA¦R?ÍàÖ’C©( šDáÊ5àÔÑ´¹O\·¨‰4& Ú‹ÔzÏ’ë¢¨ëz§bW´&ĞÇ”XÒ;ÉÆ¸}PîôaŞR9m]™°÷–£ºu¢(~æœæÄÌĞao,‚q’NÍ‚X¿˜¦¦É#¬-ie3Fü1œPæ¨‚Q;‹‘çõË6Ãˆ½§L¡<ÕÌ­ŒÄÙÆ6ËíI5¹Ä¬Å'mGRÇÁ„²a
ŠÿX˜PQHøè4ú¾„šñR0Ö×p?^·ıLqÅ1[Ôş Xı&ŸĞcŒªEØâŒ²ş#QŞl³rO5à‡”û'^WğÏ£pšCD)?r“‘“Êñ@íhÓCi;ğî)Å¤¼BŞ0=a•÷Òæz2³‰ û®Æqšğ„Ğc¿cug¼€¡ÍÓäuoXxuÆçtÖÖğ(#3!jâ'F"åyËrYñEäYîD@và.lò	ZéN+œX¾•`ÄAéCÈ™¨Ù’Ÿ 6ûŸÇAVpúòğ	ä!ÌÌT_Ó79Îq°«Üûçùø2Û|)Æa'¯«Då	ú¯Â}SˆŒsæ1=» Z¾|#6Ã=ìy¢ß#ç†œÀ~ïŒ—ª¼xœ½Z_oÜ¸÷§`“‡‹ÕècĞğÙÎÅ¸ÄNm§‡^Ø\‰«å­$êHÉëÍSÑÏÇ~ºû$ıÍ”´Ò:qs‡wÁšÎçÿõ\œ*§óêàà/Jq}s|u#2“6&¹ª”•ÊÄÍå‰¨%k¥j‘š²TU#VÊ*Ñ!‹Âl„lñ³­3Iò7ñôòâ»qvz~#n^Ÿ_‹ë³“›óË‹™8¿¸¾9;>WgÉÕû‹HóæR¼wz|sÆñáGµ'¦¬MŠNÈ*¯”lZ«ÜÇÏ×j›¤İn‚İdvé´¿.™j§MEG2^J²¸tx  ¯•Â=œ„S*¡ŸŒá5`Ìâûv¹TÛuXI¼ÂPÇïÎé
„_Ö:¡Ÿ¼şFnMÛ`µà¼vf­±â5x-t•cOÑB²
o-s+Kß·&]·5ó–ù¢¥_føËZUâÜ¹Vù#ÇÚKĞ9/Áï½"}\¢qH‚K —èî0XÃÙÅée7+íYËÀ®
^›•^!Â,ù/pñ‹J›™ĞUZ´ÂËP´HwÍ *z1t*ñ~£ÒUe
“kjÊæÏŸ‹/±ÙqL.İ‹¸y„‹—d­ÇâqMnÕõßß\^.HN¤×K§éÈ¯­²[úƒ øÔr¹–¢TÎÉ\‰…5keù˜U…–‹BÍ„KeA¿„º'	ºš+„Ï_©âMeºòÔ¼2‰D­,JY¥Ši[•µ)mFfÂT^SÆ•_½;dÏ„H-—:ÕDôÜV:•$,ªÙ(˜œSö^§NVâ,]±„ª±kF°huÁJ|}sóĞz©mÔBÈº.>:*v©7
6ÎT˜ö–ÆÊtågUmlCw'}]Ü+±××Ø8@–ZÊL±Ê
ÛÀep‡{IŒQ—L‘€X…PJ%Œ‰ÌUÃ¬Y´êhkâ‰9>>9?%†+'S¾3-ºĞÍ–\„=]‰BZ(ûŞğ	GøˆÊ,yk˜r3²'+¯&ê~Œª,MK< êmhJÕ[Õˆ˜®’R•¢!©S#jQ e[4ñ"ŠĞ¶)»QdK›$Ûë|w;¦Ü[äBW¤U¥&‹¦±‡ƒÎS`7ğ<ü<Àç„wÊ	YioÎ#F
ÈN@ÿ2B„ÄmÒ˜„@¼ÍÍ{«àşšQéP¼±j¥*§ïU7p<Ouà.PnV–£3%`9ã†Aâhâ¼—@ "EHÇ½mºÒ8À$‚ãÔ²i”­f»Sµ¤D ;Ú*ëS‡SJ]	"“Î†®İ-1³fgsAÌÀæVª¨©,ä oì@×àÿ¨QÑ]ÿéºWº8¶?1‹sé@öõñÅªij÷òè(·u:×æèyşj8û€ˆğ08Ë·‹9ıRmMuä~-<².ÒË‡½}•:ª…±s}|1À’›ÄÒâÿ˜‚«E{ˆÎ“m=CöräTNi—[L’›Ãt¢é³÷A¸ÄOÈæ{l03G‚€Ó5#õ(ÈH*ˆ7Y;œıYÖâÉsª‡öğÓ.”GŸdí±ÀòpÔ;DÌñß½b¾ïqş¢5¨)æÆæCòˆÉK¥€r@ËÚ‚k	éÖ,M¦ãö²_¯ó#¿=d)Ög>ÎdrXZÌ,×>¹)¤çòòçŸ/Çê6Ÿ>™#ú'¹—…ÎØ8zûı"§µQİ‰¬f¨ˆ±ú¾úäóHw-:”hVXóu,¸Ù{cvÌjûpÄØ9iTYğóÃ¹ˆ'…¦@A?ˆ&rŒ$
ÔµVÄsb©‹P‚ú`Du`(9£‡ğ…C°v8\¾ØŠ»œ š;vé¾Ö$ç&GïIø.VÛ>ÕGV±¤\õJQy¯:˜¹xn‰¤G7AO6ÌL(öfgEDN!WW>dqˆõÓ—5Ñ¡<S¾Ô•8`sYéOŠC5ÊcQµ%Üƒ3¯¶ B]$æÚÅ`iFYÈ­Ì¦ÂÉQ
¿V}2EswwGk¿}şÏoŸÿ…ÿÄÜ«+ÿ¢_>¹<=»üéâìê:î|;u[·!EŞF™ÎËl€a}|
KóÜ`R>±C¡|([L`úpÉ;Ã½F—êÖ¨Û
éxŞÖsDßBUİ»ñUçX¯eô¾h,ó:Ş·?ÏÕp£cÂ
ë+”.e1¹…/p[‚Z¡ú«v‚á4:]W©ª96L4f*2DŠ”
úŒ
…}lyN˜«)îÔTˆ¤-oìÊhN¶ıÎĞ(á>²E!h´ùá¶³7bÈ¡\¶€ÎŠYj·ˆvG~O÷/f1]¤å4¤´î‘¸G±Ô¾óÜ/Œau÷%#sM7-¨X¾…à¨šŠ‚«xÏ*l¾j&ë6ŠdÇ÷&K”1ãâ1¥ËÆíßE/eÊÛÇóÎ-êb?‚ŞæF’ì-n?ğÔ2‡®ƒìø¸»GŠ½µãí¥CÕU“øäÛÔIãêq®¸¦{úÆ"X{Ì£<2­ípÍÕJİ$Ş@æ[Y®/uR=^GVEoêgr¨ÊÕŸUâcî—á&ØçÜ`á­\+*-va»oÎOÎ.®Ïú_Ÿ¾=£†üH-Á]ÈŠw/Å‰W§?èæu»H\­RÖÒW03$ãtE¸O˜œ£)KÆF²«<¨­ºƒp‡h)aûŞ¯6šF^¾_tF|
éòåİî)n¤HøDï‘cÙbL«ëAúLÊ”§|İSSú¹Nñ\ŸÑåı¹Æ&gÃzÌg!øC1ÉŒÅõ`«1œ[I4ZÎt„ëÁzô)¬€æ†›•†zúbsXJ¡fjQœ-@–áç–´ì±í·ìj?÷‰@beÌÚÓ@‘®=O;Ö?:;hèà'ç¢`Yx%N}àI¸Nùœb:ò)òF¾3–¬g¾-İ€£ äñZU™ªÒ0!¸‹Ş6Fâ«^–·ºW…©Ù‚¸¥Š¤rt‡/Ğ	;&KW•£ÂŞø55ì{|4ŞqÒ<†êóİ±û#£^RPèù¨;QÁÍ·‡T˜KòÚ5$
®âcYHnç<oùÒmêÀáa}0‹İğ¶V^™¦¦ıq'÷#Ÿ>j0qo!³.rÅ520 ö|ƒ6gîg3ÒˆZÏ¤…tN/ı0f$¨dÈ¤Í¸İÁågˆeÜÅÃĞjßŠ¶°Ë Á”- üÛ÷c¥«y¨óUé9®éî¯|€b°Bâ"bM# “óG§ûóxøî¸—$[,îJAC.¼ ÄÄ¸IÓÖú1\ »gÖ©H,¥.Zÿj:I,FQÓ:„ˆ£%Ğd]à}qê³Ï÷2»ò'ŸÌåJy#EMZä)„àş¾ I²Ê†—šÇG´AiXªÒN‚=}3h½+ÓPğ¦°X(RlÖz	ôÖÊ³k­µş9„ !YFÁ9ÏZ¯ÿ’ÄŒ0Ã§Ü|ƒfw{•Z*Ò"j:
†NX
‚_×kÀ¼£ŞWÆ.t†Øù{´»ŒHz~;0ô–(‚ÄÏZ®qGcvT¬}‚ôlÅÍŒ‡Lÿİûó#’…!T·Ù*ºÊçë¢5KHråå‚ÿI¦ewŒèµÕàobd$Z0Š9ŒõıŒ¹á<öãçšÆçÃÓ¼2ßªäAP }“&—„‘¿¯dÛ¬1œµ<R N€4—öñÙü!önLúÍ—`¹÷Ñnt‰¹,µB½ÈAÜ“ıj°ÛJé<èñ	‹‹]ö¡V–3–m&Œü`•lâ¬™û¶.7ù¹àF¡¡şc;#jÿÀåĞÍ MJTN(çİH ~²íÍè1€¶é-Œv9ÓäyŞÍ¦›/_}6I‰<à¥i+ é­nš¿é&õ°âõ‰ìp…J¡xÊüÔW=Hb`Æ!ƒDªhşª6b”-gİS3é]¨Í°#åG®aŞ˜ªÉşıàˆ„Ñ†y†{vp€âå_ƒŒ£›O©¼x6I®ÏfÌŞ¡od!n2
6÷”}á^ w”@{ä†?ËÿñbDî@gâk7¤wı=ŸÀğÕƒïˆø1W+~?ºdx-3nùi±(À¿\î~sßüG'4ıß –Íh`68‘İk×‚ëOá¹µå˜ ##¥w5SCÎ,›¤GÎ×ô¹¤ÿÅ¦gê=nïîÂåg¶[sm^’¾MEÎx2“HAísLDœ#^®xC)OWÄÅ‚¸ëFsXÊNİiİe|¼)?× ­•9>”YSûæ~)ŸÅ?ããIáÑ×*Übm¿É±ªğÂ[iÿäËÊğ\zoBH.1X(X}ÎŠ;åJ/ıî¡ÛG`F2%ŒS+7,¼czËLèbÙTcœ/vÔŒİçô(ÒáJKœK!{
¥À	Ê:\øıÛ7½ÀÎ›6ÕÃ>…#C¡Ú/GN.*<v†!À–—Šó¨,’_É^"‡,¼Òy)GÔ½1ğ€‹Ù2 š’$A×öF;ÛÁXã}Ø°äÃï~;.I¹„˜k:ÚÅƒwvYqı£Á^Ï<çeŒ³Œ6Çqa±}e¦K;a‘Œ>_ÚÒÀf’h¾?œÊ²Ú’]«{âh/Ï„ŸçôéÈ*—Z½ğôè;¾Ø3SQÒ|Æ·+’ÿk2ÖK½ğxœµÖQs›Fğw}Šíä¡m&jİ&}H§Ó,.2c	8»Éœà$]9@Ifôá»€#²T4nşïÁò
H²J&1Or¸gJ°EÄ³Ñè¯Æcp©æPeË V<áŠå<jM 8Ë8<pB ãêö5Wr	,Šä`ş,ÒoñøïzEİ2¤@tƒ½1\pÉ„–ùÓ¥DÓÁ!cçÎ|ªI-¸³u’j…z	bê—"èZd°‡Hdyùšo²y
Ï°.VdXnñ­º4Uò_ä¿ŒFÛïÙAß±gi.dÒ{Õcÿ\…rÑ%+¢²wQQß"f|ä€ã—<+u,Za|Í¶=bŞûıÿmå^¥]&ÌÒ4«"ÃV©"ID²Ñà>a¯úØ°Y´ÂL{âİX.íáÔ˜µÌò„Å–RUŞW®†·©‰‰dÀ¢rY¿±-ç0&•*?²y{uuÕ,Yan(µOÄ”·œ‹1ÕZgKùH‰cj3ojy”Ìí¾¼F\-K€Í¹JXtjB’éÇÔ©cÏ¼4`ÏØÓCÛ·wÛø‡•âÙçhp6Ã0vLçydóÇ»·¿7‹>aî\âÁà\]6Æ"iTıŒæºG0)Ë²/R…ÿ?F¿î¥<ap.²E9BÏˆg³Riàå<N#¹ş3f®}ô&–iÖÓ¿R‰ÙW1$E¼Àšr‰3=IpÚâ(Èêyóçùã.æ·V MŒ¡ÏH§¨#Bõ'‰†cfÆBù>h‰%Ïvª•ÍáLNIÆugŞÜÒIët3¼b^ği
EV~G=w ÄÜjn5üˆĞ®gDïàláŸ5ÇòØ‰Ìá–-XùÉÀ“rµğ¸¢Ó™L™YSÜ“î=ã{ìH$WÀ7ømSõ§Ö]3™(ÁmÜ¶<¶)ˆDù]eèUŸ†–?„	ùfŒ».+b®üNÌÔ±îì]Òf¥d‘¾”´©–óû’¹v¬[âì´ªÆ,”|à*;#—ŞAùçû«÷ïü&ÆÔæÄµµÉşUcÊo–²€_²M;µjÙÆdòŒÉe*‚³ricråãjYÿãb{œOîš–GŒ³	\Y(ÌÆ,ÇÓOxÃÏ•ÎÅÍ¤¾Œyİ*·ıÏ+´î†xœë•ûÇ(¦¬à’Zæ_P¬à’Ÿ\š›šW’X’™Ÿ7Qì ¡^°‚xœåWMo1½çWø’ªU©–F9¡6%¤ª”¶’^‘ÙÀê²ŞÚ”¢ş÷ÇûÁ~²)—ªÍ%ğüf<ïÙ3hø±…È‡[ÁWŠo.b®ŒğEÌ#ÃFŒk6
DÆ»S22ÀpfÃ­YK%~r#dÄf v 
°)Á¦ åVù".ş]^&ØHA€OÁCÍîB¹§¯ø†©ÕâåU¿ßcWWöåºÿŠ¾û"0‰LlÔ›Î­<£777CÄL¾Î˜'9fìù"S)±Z&—l4`XŠÈÌÍSï}¢û9í»…òn>p-|Úú€%’wn§}‰$
x0ß+a€âg˜†™‚ÙªH3îû õœ2Êòµ{N>aP!mØ^˜u9ÕlMDFIcñ(‡o<Ç-æË(áÁ~ÄvaõS ciÈÙ1Ñ‰Â
ùûµS±€³ÆHõœp­÷Ru*&âK¿AÅêê3´‹²g)¶Õ "¾ö‚Ö?[Iöw¾Ä:­ÿe‹}$¨Óø:;©M·ñ¤jg{æ	êõNÓM+²Qi2‰é?<½Â¦8ß*‘«Y’ĞA90(³dçzov§—¾©dÖÁ§ÿŸõ¦n7	s“ë’&QïºŠ3¤-”8³üÉŸªÁşEË«ÿ6UuËÔ16"*Î!T­ÂGº4c´Œ"¥Aã3ø
6–±ûˆAIõ†ù˜‘³Øê;¯¨Pco÷6)µ½ÎV´Ğj¿Ú`4+ŸC“(¶õØñE¡{G>Ô¹L‘3Qf8ìeEËUò¥ûöyéŞc²Œ‡!fæòFÍÓEÕz<6ğ{‡,Ë_Í±cr¥ƒ² aM”Ûñıøa|N )lPğ4Û™";Uî$ëÁf7µ:¹›J¼dÇ“®²ØÂLÓËFô“¡¦„ÇF8+3€ÃĞn¨FrBx±’KÖõq»,SÙ‚YŠ=¥±c>ØG»²·À}#vi€Šœ3;4ëÙ6Öäz–I:_4´ÔÖØÌ­ª=ºĞ¾1y´Ã¹>)S•ïğñE,YÒáP;¤w g§#MÈìD—Eq=§]”†®Ÿ‹R&©¥Z©Æ1 P)kª¤_v½‘Ø;ĞóÔ}´C³Z,q’p}Ùuº£š-•DQÄP7ÌĞú,„3O¶Ão³^ıdÃ+ô¶SxœÕ”AkÜ0…ïş‚…Ş{m¯µ^04]hh¡IH—’öbÆ£q­VkIÛMóë+Û!Ù\JO…=‰ÍÓ¼ï½X,Ø|Ç>ôŞ7úèöæóuŞ›8ÖAwÆùM$IƒŠ-c3Jbõ"¹µ±ê¼2ı†½§É«/ò}ù£¹ÒºéïÚO»!ùvÜg×úköQËËªŠ¶¦÷Ô{¾û=Ğ†Á0h…Ó+ñ?¼5vÏVSF’Œ"o~RÏ*–$ˆ2Oˆ‹ç$JySğB`¹¢”²v‰ìÍ<^ûğzİÃAˆä\=]DÑâ9;j-¹íÆ÷Ï)øyü?&ğİBï'¤@cg×3Nà}]W¬håZ¦"å™X…˜äøZ&-Ï³¬×(—§)lµ
fØÖ’§}6Iàä¼Æç!‡æ)&õÑ*O§°·àÜÑX9¶Îsxòàlû±ëÉù·cÚÏs»>Qü=WHl–ŸK(pº·'ìtTL@Û,qµâ¢ œç(
²¼”ÅJ`“HÅôáHeÃ¿¬>XT#³Ğa÷=À~Ğtf?ÅõÿßÅK¬xœ340031Q(-N-ÒMÎÏ+NÍ+)ÖËMaûüd9Óç“™«ÿğÜñbÛré¹ vq±çxœµVÛnÜ6}×WüĞÚ€]¯İ Û"Åzwí±c/EÓ"p¸½¢%Q*I9Şù÷¡(í%¶ß‹gÎ93œ¹•†âR[©¥ıJ+]ıPM•ÔËe­Å‰Dä"¥¬,ªRK¿ã”^/ô~P»ô”V+iîk½´ŠjíêŒ
©cá„ƒ©æ¥ÌË\ú¢ts Q8¢²ë(“.$QåX‡§8W¸ï'ºF‘+A2–Ïär¡RJ¤*ïG!a“çåg:ÁA-–·»a®âŒ\I%nÃİbiD‡ˆ(¹REçõ&4ı(š¡m%øWù|Ş•Ú™2¢ë6K+cì“Ì,‚·Ê5‰ŠJ î±›±¿`'ÛhÎ‹e9’\¥pÚQ G…,v‰ÌJGø£Öz%¶| ²Eí‚Mç*œ!ûŠ¢°•…p2!°4ê‹pªÔ>¯eªtÂÀû`Sajë!ò"ÁjÎ‘@2JTpš«õ>FÛ@Z(íDœPYŒ´U¦ôaÕå5ºÙñR'_¢l¸ÕÊ…Ìë‚S¢‚0İJS(k9K c&°icpN¶’Vİ«¬¥‚ùñTø[CÒÈ‚	€TZöD4PÒÁİ"ï“‘"9*u¾:Ø9^%€¸„’èÀl²°n1Â?N¡LÉ:Sg®68ƒœ}wkM~úôI(µÔ¥‘Ñlpv5Ş6 ıH%4Ÿ_èvry=˜| wã‡‘7jwnŞÏèf~uu5…öÌ†ÇÎÒlü×ìŸ›,ds&g—×ãélp}»±"uw¸kÛà0’O•2Òî¬¿ŸŒ//n8BÚĞd|>Œo†ã©OÍîcqÇ¶{ËºYõöÑÁ¯DŒUÄ•A±`âBC]Oe\åVP¢k&±ÈBp¡‡µŠ+ùY0×…¨ô£´N-YÂ0c<ˆ„kØ6½T
.gÍ“÷£¿æJèX¢¥é%VŒ\BÚğQõ(âU5µ=~B	!¿XrZ?)HĞV, ºÏè¸Dq¥Ç­"¢¯Ñ^ûµ×§ğMôÕÿòNƒ¥…ÄæŞ´*º2UµwØÚ4jà³{,ö;ñ½ÊåŞ!…ï\¬ĞıÜŞÇîH§vzÚ;}strrtúËì¤×ïñÿ¿×ŞÑ‘u£©µmï¨w2;yÓÿ¹±õ¦ßğû1úæiÈÇ2ö=k£×ƒX…º6Û 8ÿCT¨½€‹K“x7è¢¡c§%ŞŠLj¡Æsæ¨Ù4ò(§íî;eTAš±
íUıÑ~Yq\"?ˆšªî(şn[R;ñy*\ğoá<èbt;!nÜ²BÈ¾±I<`^¬[r`hdA.Šz¶—á9~ªJÃ¯3Ús‹F!Ì¸³FgĞ0|11J4÷!!E_7¹Èkrª€™(ª¶}î zÙ"[Şã*™KôÁ&Î2)ÍªÉ AWÅ¯?G*†½½
ÆMÛHº'9º5å£J¤734¿vmz%¬­×q|–‹´,³@X#·ÁÇ)ĞæN<¨€qJ¼‚¨ê¥ÒÍGÏN!Q›`­Ÿà\“ÀgÆm¹càyíÀïôû{ÄÚ¶×ù(Ì*˜?€ŒBgr4‡'â~°µ:äÕa£>ÔíÖŞ`Ê›üÊÓTšÇ“£34D2:‹|¡Fp¹\ìŸöz‡tzÊ?oz¡^‰nJĞYÂÍGg}:Wª™AÛã„ŸÇºSÃ£·oÓ>M8?ØoO­Õ`
3v:L%·ñôÂƒgˆ±³w7åzü+kl{š÷i¤,÷§ÎjŠ7Lêg“aÓ´ül}5"Hh¯êfÛÿmaß!‘ ”}Kƒåbi\@aIº¸»d‚;P°Ü\ì÷™OQ ²wÒÔÔ–Õ°O<=‡¡;$şğÓ{…Äi½h”÷3ÿ¯‡4yP=Z¾ëvÃ†¾L­û‘õT{>Ôı&“(:§/ö¦Sık€µo¼?»tÿ©äç.n¶«ê¾Ï¡”ÔWõ]-Úïš`6º»ö
À¾¨õ—îœw¾ü»‡½ Üï£5]û5²Yğ4Agäj™5Ïc Ö`ÑSùÊi¨	xœ340031QÈÉONÌÑKÍ+cØá}şhóÎÃÏóO‰+íË	¼¶gº!DQAQ~JirIf~XeÆì•é¡ß×Û.“ªÏ»_ÿóë[¨Êâ’ÄôT°¢–¹Ç>Ÿ3óÑãHU«®`kIã‚**I-.«Œ9û_æ“Ç¯ç•»xä<Ï,şÿ =à¼&xœu’_kÂ0Åßó)
{º}ˆm¬¥š(î)d5seiÓµ¶o¿´uŠâ^çœ÷ŞäæÁM£ÊB˜R× ÂQ¶õŒì N˜ô_cÊ<¥¡>´Íàœy3×u{tÍıq‹Ş€ÎO(U£¤ƒ¾lk¡†.N¡JY@aJÄÑqŠòmä#~¯ÿØÕHÄ’‡Vv_
ğ¶‚uøy>›öbcëxb_•õAJ/*Xz=©Ê¢Õ#íğÂÈŞOáû8ËÏ"œQïéÏŒ;Üİ$‰VˆEé”Ò„§8@Ş¾ìÄ›’ıbñş)@W1´{Ëu‚C¶(côlùIdµí>,pRèº;V²½NÃoÈ™9´úØ\ËÇ(§—·zY¸‹é‰É`Š(>ÿÈh2L"¬gtSÃ’í6Û@mÅü•4ó5˜½ïîXxœ{Ãrm‚t½¿chˆG¼£³³kpp|ˆ¿·«_¼§›kˆ§¯«­±™DA«[k°º
C#K„"0áìïâŠf„²crrjqqH~vjOfZjIfnª•‚H.YGôõ2òK‹¸”ƒRÓŠR‹3ĞÔBíÑ«4QHI¬,šZZ’áœŸ’Šd¤ÄTTC7J1 ¦ŸD`î€xœ;Ïv™m‚kN~rbÎF©LÖä}L
PÖ#&k(K‘Y †¥ì,xœ»Ìö”mBƒ WxH|°«skˆmqjrQjÉÆú+L ™
Éê€rxœ;Ïvm‚KAQ~ÊF©L`Æä}L
Æ#&kC‘Y H$¬ì+xœ;ÏömB½ WxH|°«skˆmqjrQjÉÆº‹L ˜

¼áDxœ;Ïv•m‚(‡‚­BqIbzêF©L¬`Öä}L
PÖ#&k(K‘Y ¥ï±ì0xœ»ÊöœmB“ WxH|°«skˆmqjrQjÉÆÆ+L ™¢
Ñì‚xœ;ÏömB½ WxH|°«skˆmqjrQjÉÆº‹L ˜

¼­xœ31 …âÄÜ‚œÔøÔŠ’øâÔ¢²ÌäT†+sl«nKÔ-×RI½´ºaYÅáó¶ |•Ô£xœ31 …”üÜÄÌ<†ûM²Ó?ÿŞàÁÄvLtÅ:K±?•&`¥Å©É‰Å©ë]˜‡w\¼Á”»JqŞÇ[“Cóİ s‹³¨xœ340031Q(NÌ-ÈIO­(‰/N-*ËLNOÉÏMÌÌÓKÏgˆH»¼xòñë¿§ŸË¿Ì÷Yáâz `ãF³ùxœ­WMsÛ6=‹¿ã“”‘ÉÉLO½9Râ(iSå\QˆH.€ÕNş{ )~ËrbL
‹}ïa?@ ãâÀ#`Ès³_bÂeêy2ÉP6õ&WSæÊ›$BÌ®"iöùÖ˜¡<Èë=WGÊ ÂëD
…×’,æI*åqà°ƒĞ†à
Bƒ¿ƒh€¸2RÄğåõ8R¦Ğà6ßÕH}ì&xxM˜M¤1¢ñ<—aÇó­6¿ Äƒ‡?:fQ©‘øîÀ¯#R;ó<sÌ€İlL•Ãş÷&«%;ıY.³¡¡¯áöÏ+"fß4¦îí«7ùÄ¨¦€L#ûæ¦¦dª&»wš¾-”ÌHHÚÖ¦ÊË‘×RèÓŒrÅ*æâ¿ã¬àÊSaYY'ş¢xÎ(…ê³Ñ°àÚ7”ÄE,)FCs&œmµœ3B)ÕÏØô•«"¿p\3’¤€RùÈ9SJÂlÖ«§ñİï·R}ÿµEŒ+G’Ú‚à±¾U|L¢˜Aé_²– +°jÿFĞú3 ]ƒÎ0ÕP¶F%ÿã6Ã‚8Ã+È<ßÕpÇµş*<CkÛÖ	9eåä÷KFc;zï¬gÔkYæUjêŒâê&œåuÏÔ4ï©Y.ç¯)úôa$GH‡p‹„TcÔºJ·ÃQwHcÕóÊ¡æZÃ-˜¦áŞÂKRiÃ˜
şÍAïÛ¢ª`•œÕæ³†µ¤ìØŞ\4V0½8ğ˜Q—èUj/ŠM³#í h£šºT_æWhí¶ó/+n´ù¨ü.Y+—_x,Ã=ÏŞD$knêL<[É¿ºµ¿“iXŒ½9.J¤·øQH0bß[Á»@]ŞŒ6¦ÏÉÎb0ĞÏï4ÖP|­Éı0<kÅ¶àŞírÍİ£çXe]Oÿ¨"ÏÍFá~İ—\-·ˆ›ùTéÃÚzZcLé¦(È°/†l-g7Ä‹¶;‹²„Ïãq°YéíMÜ”sG™Öâ@c—W'Ú~»§ÕGü©dy¶ƒe7+
…mÛÑSÀ(Ô=˜êtQbÔF¬¬õM–:,é¸o]í“í­Ê„­"†îÙ-µ<ëF™½:]aü–ÇºÈ€]şèŒî§j ´Qå…q¶]I»S:¬U,Ş“ÅÂõ•³—¥î5¡q@™QQ|Àm÷ŞL¬âZÚG{»Sæ¢«ì.ßÆRï‹À¾}½;$ôU¦«²f¾ï»ÿw1ÒÕè¸ÈMçI—k§•aTğOìT‹ì†Xxœ›,?‘›¿ 19;1=U!±¨$39'u"¯Œ4§RzfIFi’^r~®~z~~zNª~iifŠ×ÄI“m9¹8aêë,¸ Lı”’Í9ŒŒ“cy•-¡b
ZP†KI¾sQjbIª#D (µ°4µ¸(®© ¤jò^1f}™M“r“¸´BjQQ~‘æf^CãøOæc õRVüî€!xœ›È½ŠI¼ 19;1=U¡81· 'Õµ¢$8µ¨,3y"ŸüD`	M.®’Ê‚T…`TÙÔÉŠL“yYø'ŸbæŸ¼Ù©– -}©xœ340031Q(NÌ-ÈIO­(‰/N-*ËLN/-NMN,NÕKÏgà_65ÇºïË>GÅŠ’{ı+r£Š~ V&µ<xœ…RËNÃ0<Ç_±ô€Ô:âŠÄÔB ¸"ãn‹Eb‡õ
ˆÇCS('?43;;»ÒOjàUİTx±á¤£ñÖã\yÂÔ#†\dí,ã†'BdŠØè
ïa²6üØ>HíêriÌìQÑ›Zš²!Çî¡]Íƒ´b,×nVMnøéeÊ—ã(úÛÃ¹«•±ãöõ‚;$«ª2Iİ‡÷½Obå²S›ˆlM¾ r4®üÎÎU¾Äˆ*#>Ğş'X!¿5£y‚gj5ÃG²1¯Z†x•é.>…XµVÃ%¾Şü­‘ÿÍ,öj¦üäˆLô@È-Y8q;6O`¸OEö¹µš£oáhD£€9a˜ÍYu®yı"Éy:§@øGÛ’?×øÜ¢íåãß8ëq
İÀŠØY]QœÂÀéöäŞmÅNäĞ¥\ w
ÎZÔlœÍ‹0à›ïjíh…ä^¿]…ÈÌª#œ‚5U4ùxNa»–òJ‘Av«¼Hßy`1ôíĞ:ÆğÈÿX±¶‹x…UÛr£8}^;µ…„¾ğû6 PƒD$A‚¿~ºIœ±ñ)Wâ*ëôíèôQçôh!iM<«]Çµ¹˜ô¬ü¬´á­K;S{—Fè«"ìv­K“9Ëw;¯£ñü»ûç.ÁOSc¸ş¿wÅ7æ=B2e?©ÌD)¤<H!rqLOú¨uu<Ô%È‡4ZMP·gJxK&Áğó¹ouğ.ºjl¾Zı]PBˆBû²8¦uÓTEEq*²íl×èœıìY8lÈ]¯Ó¿tRÖhë“)g–ıgzdqÀŸ*«8×Zàãh4ƒØÕ±7Ö* L‡µ+V˜wvHˆvú‰ û#°æ³iÏÑ½ç«]ß˜öOÙ¬ªBTõ…Ci§"™
&òUIk*>¼R¸ÈXùĞñpi9xïüÂS¹jÅ»
ërOÏ§œØÉVˆ€e­½r¼¹*å?‘â¸†BÛA4ó‹j.*E¢œ
V:
ÆC€[Ì8.–(Ô{u†¡9¯]åuZŠb}ö<Bˆ¦™?§«rl¬À3ç[~UÈãB–³ªo—ßßqÜyˆKs9Š¢îÃ¼0%O'$vhıPSÚ5õck½Zˆ8aô3*…¾>¹ÈÂybz+YÇ‡¡?Š¤/o‘Óß´O¦=“ßÜÄXãîÖÎ],T0¹]ÛO‹S¢[ĞŞÒ@èZ[xSè8*z3mNÎ&P¸<¼sc€è?¸ß¾tÎÌÂ«}ÿ[‘‹Uc\ˆ¨n Ù-sÙq³¯¯=i]×Ñ¢“ò¶yèTŒ=_;ë<MB¸?ùf…¯0EGV)·™êj¬=ãX#Doz\ñß›I‘K!Ë½Huv€ıAí³LW›jn¯ÅÍ˜ØoÃ;¸¢ÅÓÚ4z‘ˆå7#¼™{`é³ø†î«×ÊØvè¾³BÏ¬æhıømûo½Ü¢âí]£ùt©w+ß6I‹Z'½_ó"İÀŠ{TßŒLä[Øû’M>/ÀC6Ó-2Û?'{²è—·oÔ`_œç&z.³B”ÅA–©ÖÔ	šª–§u›ëÌ·'˜8ÊÑ`Öx|0˜éù¬:Ë&™Lòvà@·wã!Óİ/E	Ø°â…txœB ½ÿ¶Ü:4‘;˜ecomerce v0.1.8‘°½“è³^79.0“˜P“Œ³!96“€“z<ô]éçQxœ»#xGpÂCFË‰”7³2¾db4Úüù- uõ	W¼áxµ½×²ÛH².|bîKğîDœH CğæXÂ{ÿôZê—¤™=ÚÑÑ½Ôê/³™YiKnZtŞ·gQ<Sÿ›[dğ³øG|Ã¨oøå·¬ğş¢ÿ×¾êVrŸ¦h;ˆ¨-@È¸:DR‡·åÈM>Ímøÿ'ˆÒ¨,‹oQûŞ`×^ƒ‘$Êş£G¿¡ßN<D;*î3±;šäX”f=wñánÂWâÌ­Íı+¸“›qµgº‰_Â©õ„1­@6»@íû^5hÁ>ÆV#şßÿyFmØ9¯oº:oo]Fp[dér|üº÷&ÚÅ²¨¢mÍÌA^àÊ¾ó±}K6Ò&iJ¯Ü)0ıXÜÿ˜%@û¢±íj¿Y€¿>0eQ£0ÅQúƒñhÏshÊe}l’èhEB$'dCÅó.ëıÍM;»NèàT›øtôÄ¤îÈ¢º:2—$;©Gí5[.;íîv•á–$ªûŞ…uÚ²Û>¤ø*¼·»Iü€õvyv[Ôÿ<1†#8‚¢!ø‡Íz$JÚëº¯o“´©w¸7mxQˆI)ÔüX¶ñh·‰ˆĞ®ˆÙ	>E4İ†èêÀ¦½×Ÿı¨Bè§.?C÷0ç*®.äŒ5Mì*§yÀ‡
høJy?wâ"Ì›"‡İ´p“O5_AP­{731İn½5Xˆx2ÈŸâæøO‡TNÒÍhû
Úõó¦k>¢¼ië.óóÖn#@¥(ıüó?•uÑ~7×7}¨ã%‹ê"ŠÕùT9r³‘‰3ËàçÀß]ªxoŞ´Ü×W7ôóç¥ÜGuÛÙië×Ùrä±häƒ’JAuT#6Œ™œ³ÖÙG[gÃtÚ2U*:KXø2(ïùÜ•œ¶Ó8n3<¢QuˆÅQİš‚G(¾•Õ	‰“?Ñ
EêÔz‡O# Î¢¦)ıôÓ€‰rS±Äóø•ÅGo˜}”¹™Ç§‘ÜÑÉÔß¶‰Ë"ey™Ş?®›»Üy¥°ş©Ã‹Õ",B`Œ˜b=Â#(& ‚E…Q(¨}Töîş8ÄÁÄ}Š`Qt|\ò`™æß/&”07÷ı,e÷ì&P™‡ev÷Ø?zlù+øöym{1½ß®çÛù&©½zÑh“¢o¼MP­º?g'×¶½ZdŞ-Å³{ß}.¶ùD5üBWi‹±\;EaóÌŒÎz<å†n’ó‰ˆä}|k$È%÷8óÆ÷/ÑEiú˜áuª¼ç…àpô°MÏœZ58T'{ËBñØ25¥ß%ñkÄ•ÿSV£$ú ~y²½~YÓ­Ÿ•©İúÿüÌG…$KĞn8âøæË©°]ÃÉ½ÜE·¢Îä>¬6Ş°£Å‘ä*¬&±%ÆØí]W½ÿ–øJ O©Vö>Õ'jŞ6‰JÌ©Ûå^÷ÈÚBê1M¾ÁŒ‡ôş­Ö<ÌmQ¤ŸWú²r×OT	ƒíI¶-rÙ¦ÊÆRnéíº-‹İE§¹ùwaWL‡Ù¦óÒ6ÙÃP3–ÕO.GÁ:Zãa{Ñànÿ°…YzœŸ÷Å¾Ö8-Êëy[éødùë“±oáŠ¥¥0›l~HÔ>£<ÔÆ€±g¢ë[ğ–ß}sU‰î=ø—DĞ-×6®m„dòƒÇÆ èÅjDLÿk,¬İ U—çñgé/­Âråg‡fRªê—‘bw¸ ¤Ë+ì»µ®Îù2 ÷ãéçŸ×õKÀç¯İ²ÑâJk¥„©ÄÉŞh(9a† çG¸t7iæ7\-~±á§ß6À5×üıÇÇgpÉ~{¹†íôèøğÄìgœSÇa0şÅ>İCÔ»ÒMqÑEéÑU…áıbüØÓ#ÎµÒ|BPºEfâùõlï1FËô™ØAy 7F8{~Ÿ€zùu]ÔüıÇâ$‰o¯ å@)¢äe¢»¥©…ƒ»‚ÓÄàWQ”Zİ©¥Ìm“L¥/ÿrÅl#S-E‡°Dè³‚ÓzÔ®K=ßÅßåN¿’6y~¾`6‰Z8-‹ãzÿj³îİ˜{ì”!*MV8ö›´Z2²Ç²„¾Rå©%zÇ~75€	ğ‚ì~,Èä›>¼•ğÜñt™xÂ’ág¾¿â¥ÍkAD };ïĞ£'Áú'9ó\ÀË?~¨işÑãß@¾E9H@Jå8©¿8Ù¬™0ÂØİ,8Ù#ÚË3ƒÁ;ß2QW’;úÔu1zÂ7›ä=øwÉ¬>Á³¢dä¶³Û´TvÏÆ#¢â¹B;XTÖ‘rŠË†¼yš µ¦J?¼:êıÎ&ğ/‹Ê0ßu\ö½BÓ[ØïıçÓÛßØJyTo6ãè„–ø¦–ıé€şô~ƒºby6·û3;Â0ö“ Uf.5È2á˜VCgÜâ†Ïèíã'­iZÛMà×?¿s»¶÷	¨î>MÊ®WŸÒ1¹wĞTR—³™"ÕqÇÏœù©Šï·_™~]©,şİnÎ=êö.Éè"î÷x“?z*JWs¯âÃw¬Øœ#œè½Œ¸~ÑîŸñVÇ¿ ÷è¼Å=~úöj]&Û2²5íeÌºXuyÃŠ==æ+,¸6ñĞÂàïEû°oØOÚ'Š™‘;rù£òîÆÌ? í;ÅK|ùÉ ş=0ñ3T|ÂîÉô$ ğòxçÎpÄó¶ó„aåîˆ‘s˜¶#Hóú© ô°O¾â×˜+a”UİŠ*r'Š&ò§½5¶ÙY7Î¬½’º!MÅ@°Ùû4üüt!Ÿ‰)… à†ÃIa>0ÜóÂ§\”pVo›==t)=µÏ‰¢tÜµ½HºÈø®y!%!…y•6.Ø¯¤gåJëT»Ğ¬NÏõch/‘ÏC{´ãö¥+‹¹0q;c+Ã	A'C¾z–E@oºbæ·TzÍhÖÒ{¤ıvrÛÈ*c£·õ…œJ&ñèWëş	¨Ëÿğ:‚èç˜À·4t»ŞE˜(g‡–«—Š´¨å#ãf›3™Zİ—ŒôYüÄ1ù=‹hÿ 3ÕC+oÂı|ÑÆÓ1änV©äã“WŠórÏ°QÏô©"?båÅ°âJi®E2Ç˜6¡šª+éIİŞ›ÌÕ°v›'Á[ª¾5ôX¾‡ŒßËN èÃÍÊåšyÿfÅ}”¢<2.N-v‰~3¸›Lòt§oy+ŸI»RŸşb-K5k.—å>)‚)û»û•F'»Ô#ù˜Â÷X	èîVÑvÛ1 Ü¡©ïóOl¾ÇPÚÓÓds¶“m“úv;Å|"ÅãrDvvpDG§ÿä(>Ùºy^„
N¿°9ªÁAjø°£c|@K0\™Æ÷x²n˜åò{“ò]ú D_—ŸñVÉÛ¤È¦4©Ş€¡ÌºèÄ¥ÀËLÇ„¯—Ôk;ÉKa…"…¯—Å¶ë"oaòS–'©·[åÜÄ{ÉÑ˜åÛs®¹yB÷Ï
OµCÒ>Äâ/™\¡­XÔ¤éòğ	åYõom7¢ï)ÅnW
,Wgß>kP:4íßÓÚgQGijƒ(Ó6·(’D±K¹ñ¶Y¼dÊ$åõmMã5E£dvhÈG¶¹Eãá¦D&¹oAôùÕ©üvÅô)à¹›ãî+õfA¸Í´©yš«]bÆ,‡y(ˆ÷Ïvüé¦AÈÁ–8ó¥I)Yç."JÙÁ%yy\©3MİÜ¶íD§%(¹¥ín®¾ºÖ¿A®˜e3#‘Â™X
Sº„uÜ¶!ë˜·.A*CÆüÌ¥SŒ¾¸€ºt?€d›©™2 üx.¿“E—ú Æû÷'÷ú†ƒg—¥†8xºSnSÔxğ[|ƒáÎo`DªÛö3û7i¬ó$«©€Sádç|Ê(Ã)Æò˜S³¨ú1Cª[+—±O½÷h‰4ó&¬ATfƒ`\F]ã·µœĞ_EêáÄÉØ•Js¦…ZÏáL—ğ^(êĞ¢øk§J¶uÿ3ìû})Òˆ¨g<ã6~6s¾	=L¬ä 8ö"ºÄ€–Ø"÷ºTœ ¼—Ã BWjƒ^äêDßPİ*É–¸Õ†‘·D®'?«·\;¦]jøĞõ™áïšø¸“úAëµÿá¥Øf|R1¯Y·ö)Sãv˜3U#ò{ª¢€ñŠ¤›—ä÷ˆöe†öêN¶Ô›ƒâ_‡î±‚¬^7f¹Ó»çøŞŠÍ€·5ia>¬’Ûíé\ØØ<oƒ¡M6µÍA	¾Xõó'~ÚøS=Ã¶@ ˜ºD ¡úË¥Œ
Ï§ĞÃK9	¶bã¨ÏbÔí)Ù‰h{½¢·[­ÀÜş]şöŠ}×ujJe×Ò®T»”äuŒ<ºÙ¸=*Îa{r`X¼Ş©İ»wN"î¥É’«‚òí÷òó{²VŠãs*6<íÜìÍpyG/‘=ò‘U½M	ÙÔ%ÌÉË_aşù	jÌÅ+¥±áŠã{x´Ï"Š¦Û'Òn¯®š h†Š¤¤ŠÍ ûW¨¸îO®ûîJ’Ôîš²hZxÉö@?äåùPò»¿„îQíäF¦ˆ×u7ªÏã(‘§Äİïë½¾mÂ°]:¿‡k¿§¿ÑËÍê‡g†i1}7_%+À€dSğP¡ÎæG|*3‰eîx½~ò¯Y¨+–·â—O->XMûÖq$'Dİ;ÆaFŠEND;Û=Û¶›/Æœ OTÔÀH ÖÇå^14 YgQn§_uK½ÿ%$á¯j„æÏÛF”Ç¾Òñòe0–”m°ifYÑxaáyƒİ÷["©Aœì·íôY2X$#ÃüpBf¿~l3UÔíòv‰Ú„
 +!³Ö×d6:%RÄwŸıkÅÇMxêÑ) ‘ŒñzŞïË’$'œı-ÃMq8«§wo´@.¼-x}ÜR¼Z9E™ûú\^¬Sg¥*PDK¬&Üµ¢Kq‡o~:në¯*Æ{}„88=´=˜¸®Árt¾uõQ?¨Z}„##´3˜è«O?ùàgÄFFÄ²‘GeaÓÓÀ)ê:ƒNß^BñCÅ’\éöa–7ˆ{w‘à¨+¤•ì|Ù¯iÓü$ÚuJyÊÈÃ¥‚©)ø5hUéÙf`RìY¿W>RÛùØû.p¾=ñø†âßƒ
fß"b'Zçl¸'£‰¶GKïâİXª2ï@T¨ö®ü¿Ã\±K¡1m0Ü¾†V~]©1©¡í¦³ıT6¥ˆôm¸Ù{ şô³È²¥ºßC6‘©4ıtîPˆ2¯YUèf>3f$è’İc@.DØ¢¦Ê½_¿F\±zMuq4èêŒ ›|ĞVX¸;ic£a 3Bî²¿Lï%¸¬­|÷‚¤©?®¶é=ªC©¦úiÕw‹X.%Ø»s•úSã„Y¼kù?ñPä»˜ ußÑê:HqxÁØ;tn“h›[öı:”7â ˜íµ<¯êÂ{0ğ´:*—'r$B#	¢5ç='¸ö¾÷œóXgÏçöÄ],öµÌnÛ—3r‹´¨—øè»£@ñ…Í@äó1z‚H´÷ËÃ8*ò°e
=ç5­@¯Óéaô]Oèßƒéß£®X¦o,¨”!IúÜlUb×Şµû±ñ^ÚJ¥è“Ó_]İ±ïûwı<j ó/Ç|%µBN¬İÓ¾¥OæY`=öĞíxiÎ.æƒ8I
Î)ˆ&ñ~ëUù]E‰Œ}ÏÓ‚-m;á˜ƒÆ5˜  Ò(Û@ıÚ²ĞÊpì£q¸™¦ğnõ¿\1kBsI[MSÎı±çüÃ–ºÎ½±Äã©88Â[ÚÓx­Àƒ\w¹?D^._n‘Äòåòæ”6ú´ß¥­ˆŞÀ–Dr»Î¨1'´)M—¯ï¸¿øı	uÅòÁóËK‹)
ÕY)½NŞxö,Fâ¬}¢$^éÅ.ÀğwíıÄ²Qëã/¿ÿnbâe¢Õ¼İ"„IFâ|·xm†™“±¬æ\%N™»Íé7|¯¡yå„ØóaQJ6¹´ÅpL,xèz ÅÊø>è1ÃdA$ï~à—ü. +Yè!ZAbc¹ÚÅ¦h‹':Ãƒ„GB‹F}H÷GD½Ğ—Ïµ †KÓDFÀò@Û?Ê_u} ×´‚¡8†b,‰~xå“”M"ˆç,—Â$<V1!»)Y+0!
ÁÖtÕ-d;"yO·°¨MRV‹÷Û1ûÒ\7E<B"Ö{úäŠ¢áğr[	›«SkÄ™øÛQzÊÏù÷O¿2€œË‡|5|şúÕ$zea+zrâ®¾“\i 7ßx^JÕomzrª{j£\p³Æ‹bï¾¬Œ@`ì»p:?®Ão(¹L9#pçÂüYÑËzGy”ö©NRÜ."ÅŠµõ˜+Q½ß0¿F\±ú|˜Û~ø°t¸ïî@áÓ¶Å„{”U™¦ß…	Ù‰z6‘÷ˆ”@®]şÕuf…ÕTì®\Øº²s·X[µ‚ªç(bĞ¤±CµB8'éAäÂ¿ƒ¸bõ1ša‰ĞŞï²rHØuwÙŒ!‰köémèÉc'[ï©A™<Wl‚NÇ
d²Ãì7âÉ€d,ÔK´ŞsÛ.0p Ş|«zZ“'é$¤_äú†ùÙ!…<¢óC‰b(ã‰®š®&]xå†úHêƒ¼Ó†öËÉ¿‚ı	3Ó\ éÃ‹‚ \»ŸÑÀò‰ˆífØ
Èi7]‹ÍŞ‘A3b¸é—‚WL•Ü€nvhúp¸ëûõRştÅptÜÓtRˆ¡5jG´tÕ<9&U ÏÆyÒÏÇÉ°ˆFİZø‹ê"óÛĞï0™´ŒÈü`šÏÇüåPCIŒ:| Dà#ê€~éº´;*Jrˆ~i¶ ?Ö('vıL*¬„Ò2ŒGw­µó‚]ÛVï×=è†p—Gÿò`Ä÷ÌÔ½>1™Nk+A0{ ¡<·ã•Hƒöì’JEÆ»—úm%ñ„ßúZ^ŒRñ$hFœœô®¸…Ëlm_wüEwGæ]‹ëÂi>ì.•”\< Nş^¡0=uLzç0·9#+3|zùD6å­ıŒ^âš¨¶ïåì_®õ¯Á÷Ã¤ÑzQT<Cçº´gNÏ'ª	¦9ï$Q£oõìºkšºhØIAÓ.¨ÁPİô9ôÑ@jr}ö8DRØó¾§´ºGÓuk©#oÌ$gşÙÔŞµ°ÙBšÎKûä¹$ä]J¥¿‹Ä‘PE³‹ëêÉd³j±uR¯ra”>2;“ú8©­˜<ù~]ş=òJ6ê\•ys¹b}’Ş3ÕcU~néÆÈÇ:”ÙF1 =ù]É~-£q`ÖôôûsÂ€øF¼~©š#•(9¼¯mïjkø€ÂdĞK{²*ù¡iu;¿^7ïÎè÷¨+–÷±Hİó1D.qšÔäÎxŒ-Å7q<äYhœ]UÈ£â1Õ{°ÚDuW‚´ÜÀ§]¼ VÀ­¬t©äµ­Î*îIoÊŸwãj‡&NşROê,û’ıı˜ıö
Ú½Î×¯”Ç*¶p‘úì¶ãl£Õ2=šU¿£‚øÄtÕì¼Ûİ¯W¬æö^îäb]ˆx*XiøÒš;_ÊäĞDf4’FT–ğ¸P„9Nm/çÿl–û$ÜG®Mp;(úµŸmìkÛ ªz>x‘y>É„}|ÑßŸĞV,ûQp;/¨2Ù²™HdèÜ=.Îæ‘d}mÌ£äÏ'<Ù²ïÑXó-ƒÔ~•ÁÄâè£	2DıÌ_Pb&±-`¯~büìI-6tğğí±‡÷wø3ÚŠEÅ½R~Pë™àwãqŞÈâhJĞ’ˆV	çrlóE“ZPÇqÃ.œ‚ûz«ĞíÅÄbQJ¡zğ¶Â%}n‡®Ús-0;•Ú‰#4£ò^E çWØõÿÇ`A¿`%«ßKH<aa¡ÄH7‡FÃ•š›{Ô`­:ÌÆíÖ2¬õFşÅmë7m¼*G Ÿ¿B¶™s®º‘Cäaˆœm•3 «m:9zr¹Ğå=˜¨d›èÆ»?ÿ2¸$VÈ
iJ»1T¤ ßñÃ}ç’Î#8m°ÏPİĞŸb…÷Ò¯ß¥ÓşÎ/º}uïYêİQ æ™öÄ˜1ƒî1èå&[¾Kf_üáo·’ıÆ3%VL¡º*	Êª‹ ³¦mU5ëáï,…Á!lqëÿFÙŞ×Šñ'‘™7§S¼z‘GiÓ˜&²o1zæÁ-µÏıä¢&ÅJ<E‡¾LàÿJÎ ŞòJßï=é×›X|˜”ÆJ¿jø­ä™I½³/ğëuR‡ğĞœƒÃKş…„Ì•ZÔXRÊŸáThg™Îp=Ø¤çÜä»±½ˆSÀğ,áÔ—É+07Ù éL­†R¿.ÿYz_¼Ï®âH‡Z°ÃŠ;â±JWôİ©#¸ÃíA”B×»¼ÛÈß¯§œ«põ0È#Ìe|böhÜ‚èÓ0'ş@íËs'äôŒğÍ{ªı?°›öŸ“´KGû•l§£|ß!€_AÓ-e¯“}¤î‡Q¹§×‡ ÷|¸9óÙu‹È_Šd»bú¸—ï-Ş'l¿)?Šo=œìEéô}ƒ*•„cÔC·èı½'àÒI¼ ÌX¿*{‹o½½t3Ü•Ç†VÀö4bèNB7“^Ú»¡S!ä]¿Â[±/ÃšÅSó³çõaäc&Á£‹*Îåì;Ú V®Fª[ÿİW|Â6nm¿†ñ?›ÚâşNJä†ˆˆğ¨*X™PÇ
®ªj±g<7İ!€t}ú2ü¸“ª7Â¬9:9!ˆÍı<R*mÕ“‚D"zÂ”sÆãÉQ>½»¨¯’è.¼©ŸkwY‚OOüŒrí\!Å¦Á53ƒÈ‡ñL*Ì®€8/ì¡-Ô—Fêï0Wìf—]×IÖ0Ó|¦bÆÆÄÎ¹V]kŞĞÙÀˆgW¨§M»ä=`˜ºh©p¦^–i–O„ºîåàáyÎV7Ü£SDQŸ;í Š+Ñ=ieh…¬ç»Ö8o‘úO¸`îkÅîƒ]ªú+\jÒeĞ%¦.#„÷›-³¯†' ¨¹Lx³ãD¶¶ĞÂüâß:Ç¯¿õË9Yä.x¿#áDt	•pæF=Dü
ÃòrÂ‚‰·G=–|ä—=˜S;,&°2õíO‰¢k—ÈØô%mcû°ïäÊ'%©¾Ônvs5{]°Õµ„à^#£¿Düìc`G|æ¬,×N•AãQ6±TÃMzf¼±PØ%Qæ{»vÉ¢~‡´’_ì·Df÷ìá ¶ª`SNz¿{\bo`„ÎåÔëŞOfôN9K˜´Ìº´]êK‹üŞG‚\Ïõ©|Gpá ÍSÚG%rT7×ø0‘‰%”V|Ë{ÉFKºôÈÕ		û4÷+6<î©"Ë…]ôt¡ÚÇJİÎ.°©¿,şíoÀVgÆèÚFr5!'½ë†Õä»ıD±msMj§ÁÃVƒí‰A¾hÌü9#ñ^ôŸğUˆÜtÇİX—o­÷V-‡åvÆÑ«-
æØ^­´‹ŸpP¿ÅõÚ%´vµG^oYZÊJ)A«SºeÌE{\8Ç:¾«K*ÿkœÕ)w¬e…v5¡A%æ‚e)®i›ï:ÈÎ„ß¼pW`¯/»ÌÍ½ÌbE©ü¾ZõW©ŒBbJ£Ì‡6pJX{fzñéé
5
óÉñÉä :è¤PRèUOv&S•^ÓH´\Î¯)½ßRCÁ’X3 qòƒ¡=—MT”$×5EOQgûj?[‘ÀèYô];±—›1ÇË¦¹Êürü5A(Ÿ4é`ˆÙîº/ÏÒE¹“r
Ãİ%ÔëÜÇR&t¦–~vG§nŸúw£ûIb(Â‚G°ŠĞ`
Ö	l’¢Á~èú»õ}¼QÕê—#9y›½ŠÙÜ¨Èp¹Û?¥zÿ !Îy¹Ÿ_RCßãiÅ=œÚVG‚oêü`m“‡<‘a»äÎ)È¥Ÿ>'×`ñè¿ş9ªCqÍñ]bFJ2§w’Â\yCŸG³÷¨ÌŒ]{ÆÌâàª¯¿ßc­Nœl¹oÍ0ÒŠÁ¶‡sÌ:ù½¹8ãp¸Rn£Á#q”NÄ­Y†)ÖşøšœüKMAAml¦ là;~À0öº¢ÇÛ¼)IƒL9 $
X4v·™¶cğ8rWWñäºÄ½kRi”¿ÚèŸ´˜eeqlÁ¸‚R×ÁQw}1é†—oÑ]F{õÄ[Z¢ê]€²£+™S	ï8åÙ_ğ-Á0¥	|Ù¸• óAº,µ¾\?,º“§§Á6ïn?6
iWrº¦à;ÛÈ‘Sø®¹——âåsWJó-,ä’81 Í‚"ˆC9ã¬ÏEİLdÏÇğEÅ§ÕîÓ§néúü³È°…+©óî3}òUA¢ä·”`Á:,
6i{‘Ş2¼úc“èNkYŞşi¨¶A®MŒr|Ëú—•©Tğ %‚G¥
`pÑ³¶BªÑã¡ıüŸpïğ.ÎÏïAíß!P[–„™†rIÔ÷0ÊqÖêHúH=o¦Ç¢Æ‰ğâò‘DŠ1€ß+Øş¸Ì fÖlã«!òÿYF§½ñvßŒş8n.·s­&q|½<ÄK5„'YzØÁn¸ÔÖÖûoÆ€Ğá>$NÑ¸O `-#W2“Ñ¬á­}İ'Û{FQ‡¡ÕCğ Í†¡9…q`vâ_b°e>m@`Ú(âÓ…3Kgâ‡Îü)RÀìP¡À$òƒãP$âRÀşWHáàÊcp°sğá1˜¡Bi
\C+R-»ßµ³?ŸFÒ×°ºÍéŠòµJâ­J7òcƒ·ádîeºT.~ÿ­Àî.ØŞÅ1`ä¾C:®C¢f¯üÏ‘{‚à‚ÅÀe‡;A¡¾ï2äú²›I^54MË¸EÓÚ,Ë;BœørFä¤;Ë“»ö`6ğßjàB¸cp½·:²ˆë‚…b×+Œ ‚"8h±É€ñ·Yzm¦MÉdÈ~LµG†á>XšÑnø9¢
¤¦v®Şncº­º¯ŞøÍ®–ã€S«Š`?|l–8B¢o©E†(å‰æ@'õíì¦ll™ùÉ<MM­6«n¤¦”e2óşV-€ó#W 0ÁpúÃFIÆ[ PYğ®†g‡7û„’¦cnÊİø¨D›Ş·ô}P‚E
6I•ûÕïê=MÀ´…lÅó=(·Ñ½Ò?jŒ4yGÖ[²~ÈÛT—Ø&şÍ§ÿRwBªÓîHaö"~byÂzòËn¡i;§D®¨‚[ôÌ²nûI€vÏ:Æ<·†ÛuƒR—Ó%«Ú«'æë˜ØĞ§B•õşÀ¥6şz}÷GÄÄ¹&zÛ>”Äa¤ræ½«nKÖ\ˆæAÌD’!5åA0•éo\ß´òØ°›”‰v#„É¡8&g¸EşÄwÂßCœ,1ıª]…İµ!¨ö ¼ïCëÕ`8üæ/æJàŒ³¾O0âS÷4ëüês»±8ƒvvÃ\÷;µîôkNM¦…Û¤!_©5S²ë-l”ƒèš >ÀZ*Ø'~Ü}{8@•§?Æaœ;Ğ‰š@v~\ÆFzöáh®74r ï,`öRÑXû¸¯´€‘ J£3š¡IY?RğÇh-!€0† á
4O|°Ø÷[}±?H<£€2 }@>\ôXÒ&p›Z¯?ElzÇRc ” ğ–ı1Zy¤|8,N£ş[èóßÓzïı7x m¹Rk|qğ”|,Î>å£äÛHÔM»0Û~;¹°'‘£‘a¶I|ƒ€;Ñò©9ñúÅ‹İh¾ê«àlœ¬H-	‰‚{\â ¨ãÙ¶Ë¬£„?GŠ ª2Ñ±4¨;Ğ8€A¡•R‡è)6]Ÿ½«òî*4&=”ÇïØm{JùÔèDYxx`t_øê¿¿Š ”( Aòƒ%q×³=×fØõ}ú‡H($À|hŒú<ßsiÄY^/ùó§–³”0@87
Â¡Y6p}jíşÔ©P£’  ÆúAQ¤Ï¸4Â½uzX\’°D•CÒ¹î—üÆMH(ãæàÌj‡+O§§­y»­ô~w¬+Ù5‚‘àlàù‡Šu½ ğ€©¾EtŠ ‘ Ç>q£íÙ(²Vö?G
lÅ-–µxUÄql–°ğ´ÅJ-ş ©ïa	Ş¼rÙ ø:dóküs¤ ƒ4Šû6p\lá,c¯#ºÿšÔû²Í÷ ş×pïÉî·ìüI•À KİWLš¢£uáª›é±—¼¦šØ‹»ãî£ÛbĞ¯Ìƒ¦_‹ŸtZ1zè—('_õ’€
¦5
æ|3›24Y²âóY`(ì+Ş÷'¯~\m‹éÇb´ç³à™(ŸõŞ‹Ó·€WÛÍÖÜëÛäÛˆóyñ$´Çk™ÙHòàlvìd¯~ÄêjûBk)Œ‚£Á4‚n@.°”µ‹AıxÇ’²ñt÷Î¬«Ûßô§ª
x¥½ŒÁ«>şZïê+´QîÔÎNzŞ¢™	Náö]‰Á¶|¶¯BI²]I3_Ë?x—ûxi’=‚D}v•]îï8Á–¶yÁ[´M,Ñæ`Ùü,¾h˜¬eM.‚—N”ÃOhmğèèî5tr² ÇÜµpót
‰¯¡÷´Ì…Üz4O¨Fİº’ß;÷ùfŸ‡ô¬¹êu(”ÃğWå¢[V:)Mş(sÄµŸ²;×û3YtG.9ÉMé¦v3RE»}Ôìü¼Ó©RA?ë(‘ÇÉ*Å„J—’½| /Jq9ì[zšºáü3ƒß×›ğoëù(\øª¶GÃœ¯á¹İ(K³`[ŠTTŸÙ™eÆ­ëM·{µÅ×Ş?ù[÷D;ƒ`j~BÖÁ”‘¬(pgJá[°'•:H’ñÁü{şÖ÷‰@5ä€ŠJVGİv–6g6`uæX ^ª(»RÛ}peÌßá½¬¶
¨UÕQl¾‚Çä	ø™ğ)˜m9ÛŸÛ:nîÛÇÏ¶ñ]~ïHùè¼¼ç¦½æQ)Ü[&Å}IiêÖ|¨°cà~&ğ¬ÿï‹.kı%MmÒ˜g¥™éÀáN}R¦ÂŒ°ÉƒWq2f£ÈÎÙ_³éñûâ.çéÊq	)Éı+BQMSªÓÁŞ±X5²è)˜İİpõ³™}?ê‚´ÒeBÚ ©)ÙùÂ˜›ïFı|Û“Dã^Ì.æxÒäêÉkŒïMW@«u‰Ï_Y8+h·š(<IC‚—
¸£tI‹eŒ“s¯Æ˜v|toÁOåŸ@+ÆğM97àı¨ğkÁŠyFiEü&†,¨Gt£cL`pô'#ûç»f¯å	aA	[ïû`‘À¦ÌÅŞóÆœ>ñçnà\˜GğŠìlM‹,S“«ËcÇ/¶z…È~ÒÈ/ÄXpo€ÔƒÉ'XD€V'û“Ä@‰ÄŠ åú …´||šdÜ1ÖJé1Šy…Gbˆï©.¼;[œ”{ï¹şiNu•™c¥ÿš~­‡ûÜAeÚee)a ,hãÑ¸ãyë¬HægcÆì¨ñ~“+^ä*±Fn&úäi÷˜ (RíÔ¯YÑÏÄH0šŠ‹à›a.bû ÉD¡øº¨®>cÍ›û#ª¬ı&ïv÷@w=*ì÷ÜÌc#ºÒÌ¸¹#ÿò›-“¥ˆÑökP„Ûëë×0ô8+¼wğBMş t®ÚHù˜›±E› éİ÷œğ“İ}U%7Â@Åd|Ëˆé/
­¾Ù#¶ôwÚ3 OG~Ğ.ASá€èŠ˜ÖõQpEP¦ûû§•H!1†p?õ#«yv×Xb÷“oz?(Ø‚dìk0àÕDà€Yšõqc×I‹Ÿ=økQ;¼NôFg³?_î3á›èğ0‰K¶MØNöS«ü):ûALì¬[ïáiwØë9x—:•Õ3Ø’\•‘|İ@û1Î{8b”ÖOã¿Tî=ğ¾Ï×¹ìáO³ks¾H„ÜÕñÄ9ı±‹˜ô¬ã,rµ¥¯qÇøãÕ¸1$Ê" •±T¬@i³éáBÛöún•`r¦XğZ#{´Í?¸¬r¬]ªÍÆîÌøZÉì|	2k™S_ûß_{Í ³YŠq ³AP¶ç1ìúêı³ä^M;„Ä©ÖñÛ)>I¬#‘?Hè2œ)‡~¾J
nÁÚJÿCrËë6àì¿>»{iÏhyWòëš 
9gôØpºz»ËS—È4œ9x‡€ò!=Üe;¼ÆÍ¾¿©ı;ä÷ñ„±t*ú®á:òÙJGËê!3¶x¯Û7u¢Ñ°°!ïBÿ*å<ßÆíßV@]h	rğz)E14â‚ı[FˆNôi,8y—Eà-?šçöD}Ÿ­ªÉÚ€AÎTBÁd¯nÆ¿A”]<(HšXÂ`D`È\rí°·ÊfNY†{ÎÄf+tÇÄŸŸ]4¼ªú`Û£Amü€=À¦Ñ¿I¨¨>£4‚`|—4\Qk¢$(ÚgÁîª°ºam/›fÚíü™H,Æ±ËßT×àN{üo‰‡¹Á.£Õü³.¹¼»´®	
tÄ<ĞF°0f‰‰îFøÔLÓÀ“İ5{ïJÑ|øZãGÔ9.“ÇÍ/réå['æÿœøÊ¿ïî`q¹_Œ‹MwúAbL*[åÑ]ÀSëóò*ÓG¯i¡Ÿxx˜Ã{ˆœU‚$­#î»cX™:ëM¦jk~¦İ,åÆÇòfú¯Åú‰Š½'BÙàÎ`”_F§Ñ)ÈD”›ñÑ3=§ï®Ã}sæäÉvg|u±~Ç+Ş›YIÀÅ#H¿šYHË‘PD#>òDÇ©*¬e¡ÜéêF¯7~²İå¹£Å!|I+'§s–é¯6-Fóí¹‡TÌ‡š·Ğòv¢÷Í”œ´W¼û[^Á¾ãŠ×¨2Š{‹ãÍ+ØÁ•
,¼Æ`’¼îñˆŸq)æâø·¨ÔçZM!‘…"(25sW”sçÌ.Ó±J^´]ãø1Q^·Ëï˜\àVLBz·ÄC|İ*G®}ÏO¾Jğ÷¥é]òu)|û Ëõõå¯Àû"T<¸X
ÃÕ´#Bvy–
-P$oH‰f¯TÚ>+9,Pµéqf¢W|õ¯ WÌ²GŸ¤Ì@2íP¡o˜¢êœìøJEj·<|IG}Ù Xpı&„_/ê|ëÑ:
:ìÀÇ1`nˆ‡‘>.$dílø‚ŠãÀÍ9gKIU2a§‹áÊ?@y:.ØuÀŸB`’Öü÷O´^ƒèË¾7)àÄA‚ù &ğhğú4Š½6Û*É‰Ñ ¹œÒÇ³O¿rjÑ'¯Àÿ|³¡ Ì:jÌ+xl~‘şNd%¼ÿÙ&;K¿}¼ü}
:”Xü¾É*-j6"Ğ¼q1ƒâß>ÑÓi¬†ü©¨±ë^¾è“õ7¸·Øï¿‡[ÇFÿ5wàb_tyËŒÏAà-š·ú²°œh63‹rö ö£MSÁ€çr»ãkäÇZò=@PÁNB³ÜÎÌ†0ïÁÓ¥›¹µKá¼Û‚ı‡s¯âÜå¯<çMhø÷İÖ%—å2Ü~‚á!6À(Ê·YŸ¦×Éâ‘è¦„é/\ª "¬ <áz¥“™®¥GéÙ	Â+çş-©% A8 òĞ¤H‚´)¡×Iğ õzW5ŞÍÜ^I‘÷]ï¶‹ol²Ì<f‡Ü5ëp!ÎÊ¾ûS:k×ö±yîàOÿ•N€èÿû“/Yƒ?ïcÃÆd(tWÖUxº#6ª|Qp0C.à<ö/±Qap•ö×Â‡„­Aè÷/ˆÙôbIä|VDã	znÿSbÿ?Sá<èuàxœ­ÔÉªäTÀqZéAQ¤A7ŠĞ+A
+s*%İb’JeNI‰(7óP•J*S%ÙÔCî\ùÊ}~Áp#øí—ÆêKßk#½9pçwşç#ä§ç÷şøíŞ/>sí$v3Û}R}¨?	 ¯ó¨Àf&¥–IjW:¤½²Ìä‚B…]ìó`WóÁÄNÓÓGŸ^<x¹¡ ~Ò§Ãˆ
4‹zN¤Y±µÆXÀgaÈz¼jŸ3ìêpzöèÓNûæUmĞ1J—Ù±ŒZ!p÷–óƒµq£ŒT˜Âù¢×––ÿ’¹5x«QDÇTh‰ÊP¹TâÂXsìˆŞÙÈ™dkš¨bNR2¶<isKñÿ£._<Øß?n?ºåIõÑ>ÜAx:ÌtGXc›·B1AÖT@*é–/*§GdºŸéÏŞ÷Ã"(­şUÕUÊW7¹³¼¡(Û6™úc>#SÇÌ.89ŒAÅ‚9‰Íâ
ŠÕëÙ¼pşaáëÊ@]Õ\²×ü¬Â{Ö%ä1Lõy !òX4d+2ıÛ¹§×ŞYf,'@UPÌÑ-7˜3‰Ûàë…Ø ®ZÆMWî(g.º™Ë‹~~çòâ£¿8&©»+Ü­»EÖôÃH®v],õÁUrÁ§ü
¢øítÄÅ	¸ZájZ´SgïÔæ|İã“şàYî¤!T?n‡címÚ sÄB&¦ãÈz-noGF`n‡nÚçÁc•É$Ôfd¬s¾UİÖ­]³ÜzÔXAƒ&í|jaïÊ=¾!|N#³M––TêÉ…Ä4&ƒÉ^ÉBÖ"{û=‡OgƒÅowÇ@€ÜÙtöñeu-éˆi-9İÀ8Õ›øÈf0iÁô¡21}Ğ°õN]9Œİü·z£Xóæ–ìUÍó@Á
,Í®>=¬ÆÈÑŸÌ sËš`ÔTïÆ_3îpÔµ$ŒáfSy\š6’„aæjòB‹
Q­¹z™sƒV¶_coÇÏG>çwš¼¬”`!ú*)[b£Éõ~BÑ`ë7u»Â°­Ñ®QD'îè/2³ûûü›îyÑ1 s”¢{ğ2[À&kúŠ÷ä*qšİvqºMş¦îùØ×Æf¾¨2lÌ¤b»	¹šynZµ3d>‚ yùöç—Ğğºk_ªáD†XaO9l],z,×öh)d«ØIãÑ°MGìæôø‡Ï‡øğ•k«&r<’(~€;~:J¢|Ì*&·­h­ı İ9p2XãôëwŸœ_|ã×÷ÆŸ}½(0œliÔ0=oV¡eF±/È”Áªµâf³Óãï?î¾{uôò÷‹oïÿp[ë‡$xœ{q“ñÚ5Æbì–
†VåáîeÎ¹y™ÅA>•e®NEE>Æ9Åé9Şå!†UN¡îÙşFŞÉA‰¹“¹¤&'°‹›[ê§çëåæ§€ôû–fû†¸øDXjç»X&Wyd¤&ºe¤›T{%åæ{TU@õsN~À©½¹S6ÖÀdB¨q°e kIR„³AR¤_YPIr~v›q…{‰OXfinpdExe¶W°¶¹cèdÃt‘ÉÒ¸6ÿN{Ë
 ÑE2».xœ…RÑnÂ0|ç+"ŞÙğ-“¦\Û7Éb·”¿Ÿ6i}ˆ|×Úw¾”B>îŒñí‰p°DG#e‚rˆ7êöÖ˜ƒñ°Ş%
QŠàÎö‰Ã˜	ÎZ‡ˆ®³Ì¡•a±Ü¶UÒÉ–‘kÆOqbø
fû"nÙ…°%NÉ_%~Úñ©gjİX;±„¹í²äT„RÊ]ÛÀ¥>ÙÀÈ+êÆµ4”5Æk0pµsQ…ÑáKË)B==åcr²Ôº¤¿ÅÍßtßï5!µ8£HÃÙ;Vt‚€³uØíÔÆ„ûµcq4ùvÇ{—Ô<ë—†ô(–Œ¦Î:êŒ+¼éÈó~m<”‰“níÙÊp4_ª#Ÿ}º³F'µ¿ìñlCyßö·õ¿oô!Õ…E¦ş_ìBÕy¢	xœ31 …Ä¢’ÌäœT†>æÒ)çÙ>«Yîºø-ø«jÓ~ˆŠÒ’ŒÔ< ªÄ’Ìü<‘FyZS÷3ûßß»ö£LcFÿêbˆÂŒÔÄœ’ŒøäŒÔäl†Ï±Şû»çñıÙ<ñ–Uxùïˆ²|'N_™s*pqKQËû´$™¸Í7_] ª—>éî€xœ›Ä8‰q‚¹ˆ°´ÛlíÓ>ûSö(ï\nµöÉÛ­®½İ³îQ¢xœ31 …äü¼´ÌôÒ¢Ä’ü"›î&Ïbák}Z.qH¦ò—¦˜€•¥¤æd–¥U2Üøftôg¦ğ¬­jaŞ·µnçˆÓƒ*ÉÏMÌÌc8xEq€¿¤âÛ52µÅ˜8Ïı^UP’Ï°ãÙ¡õg~m>S9#v_óÂü¹a;BdS+’SJ2óó'$Æ;	6/Y•R«¼ùâ)Şú+6ß!j²ò“¶4—G´›»M=ŸúrA¾˜ï%»ÂŸÙ¢Ô‚üâL ?*Şòë^‹8ı~Ãc†iÿ¾4¿®ø
QT’Z\RÌ0w™Õ©Ç~ÖlO¼Ò,•}ßç-æø‘/-NMN,NeÈ?è­­sÛÂ`­K{ŞcÙWÖm/ÖM Ìö~(£xœ340031QH,*ÉLÎIOÎÏKËL/-J,É/ÒKÏghØ½àæ™)¯–<›=±äMªĞ\QŞ= ûèÈ²‰xœµ•ßoÓ0ÇŸ›¿ÂêJ¦6ÑxœÄ+Ó 4µ+×½¦¦©í±2íçœ8¿V:}hìØ÷ùŞî.%;ãÎngZmdî[Q$÷¥FËâh4ZY¸·ãhdø¾,àêŞ. ï¤€/fÜ çÒnİ*zŸ­åNN·|-³\O÷R Z Kn!# âEV³–´_šš–9‚p$äı¹ÆROuQ ş†TAÃc@Y)¸•Zek(äà!Ë	TŞ[[ş'•-¡ƒÊG¾Ùñ[Ôk'^XdçÉYĞAîŞs©^D§"ìJm$•Çá%ĞØÒşªé{XIİÁ×óÓtÊ¡Õ+·™R=joüº*OÀ¶JŸHİ“JSÕ—(×ù3ü´Z¦m‚åªºO ©6È}R¶Ÿ« ¨®/Es%Qd%0Ñkbf,:aÙá¿³¡RzÓ.«(B gÃ€Ò«Á6zŒ¢S‚}†ı©“Ìiş„ı)?a]§}
‚u¨Ø«~¤R\0)z
İò±u7&÷úf	kà{ÏÂ ó’ş™0@¬E<™xoØ‰a˜RR¿?ŠEÚz•¶wüœ÷’h´L
’9^ YQŠ´:ş†’r˜øµ]Ì»¾õ¶İ®¶¾ÕÆæ†L]]¯!½QAŸ°§I™°óT’£,c~ÖF£|8ÌƒÀpÄ§u9…]œI{8C.Eû–‚‹ªIëˆšŒòìÌ!’u÷2N&lèOpÕìhäÿçÚşºôVÈ+±ÕşSÑaı›e,Wâ„4µ+ãw/)[n·q’ÔüãT¿CÏ¦âøjín<$'mšâ~H!VªuãhÌ¿Óm‘ÍÂëà@µîÙøº";Õœß:ú]¯:Ş½òzÔ€O:Ï½íÂr´ô¶5­dAúÏ¶!pä„Hxœ›$Ø(È_˜œ˜ªXT’™œ“:‘G™ÊœüI•£$5· '±$ur “0TB¿ÌP‰k¢nTË%¨DJjNfYjQ¥~zQA²'TƒGIIÁÄrèª2€âU“3¡FNŞÁ(Ãaºäç&fæM`4‡i‹ éÓcr‚Ê¥äg–äUÂå'Ú™L>ÀÄÅUQZœšœXœºÙ•9„jÛæ9,0öäå¬60æ3VSS-Æ¬gÓ€0Ãõ‚RÓ3‹KR‹¡’‡ÙÀÔu±kÀ˜[ÙƒàÊÃágÇp˜ Œ?‰Ø®xœ31 …ô¢‚d†şUœŠåïØ,.m¶Ö\ÏÒø<ô®	X:£¤¤€!!³ğÜëƒ[Â¢y>sŞ2½›­ºq#D:;1-;‘á”œ¦¬ÆŸ“µrÕZç4ö¯œ¬œ· …p$É¦xœ340031QH,*ÉLÎIO/*HOÎÏ+)ÊÏÉI-ÒKÏg8qCä+¯ËyN™¬ÓïûÔtl~"  J°Yxœ¥TMo£0=ã_1Ë¡‚Šõ©‡İ¤ªréaWíİ5j05C›hÕÿ¾c Ou¥rŞŒß{ó!×J¿ªA92ºÀ{Wë…­ÈÙ¢@'„)kë"„šaÜP(ø;·6/Pæ¶PU.­ËÓœ¦ÚfØ„—ã)jÏ0È=İ@˜ziŸ¥¶eš™W3{Qn«2“ÖÎ’}n×3Â².ašÛYi´³2Ğ¤ï7{¤K[*S]&>¥1\™«T1òeE8Q’ıÙ‰ìn£±&c¿gw,¡ˆ… m ÇÁAC®ÕEĞ6¸PÍ8à¾9ò±GÅ§ë¶Òğ€ÓØ£VŸOàÃmñj©u\MVÜy˜C«|ª‘†ë)3†…C®õg¯iÚÀ°trÑ¿pø×ãöÈƒ¿ñ­Å†bˆ.g4µ­L ³¬Èî”îü®¦YŸ¥eÜ×ò J.dxØô W,±ÑÎt#™wtå,æ¼’|R…É˜ü@‚Q,³ö¾àÇ-T¦ğŞvıäß×çĞåÀË8'DL{éqéºŠ½	-‡iÈ“n'Ã/}p¨§Şû‹fûó«Ì÷lH”«¥üCÎTy'cKwÁı~Nè^w}Ooåâİ#~mWÙ-Òá‘³›tœrºJû]êï:yçƒQw1ÊÇŠ¯ÓKdYae	ö0æ‚şŞ6íÃ¨xœ340031QH,*ÉLÎIÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgp¿²ØâW÷î«Q;^õe·8=o¹%¡lˆEWQ~i	D‡†A3£yË³vşŸÁá'¯Ñõı q„*¨±»xœíZËnÛ8][_ÁzQH…#mf•A©İ4L› N:‹¢0Š–	K¢†¤š¶ƒşûğRo[r?
§M©D]Şç9—$ÁdŠ8NÕüB©dÈc%xRaY,J¸PÈ¶zı€©yzçy>[°“9ß°Ï¼€ŸDŒ~¢h”„XQğxÆ‚¾Õ3*G<Â,F˜ÏbEEŒCÏÌ÷|£ P÷û4ÜY›šÒEÅwÑ¨xÿQùI'¨Lx,éÒÌ€ó ¤^š2éKˆï¤Ò¥ò(™sïË}Ë±,õ-¡ˆ”õBR‰”(ôŸÕK%b™W5+{›Y?,k–Æ} ÷U­í”´	;Á&<À *1zY9¡ë§(%«÷£´h“š·ºáK›¨¯¢rA5ıªD…à¹z‰N_£"[®vù¯Éå‡ëüİv¬^ p¬nt"Œd„“O:,>ƒÅ.İàdã‚}ÇŠñxJ4"ú§ˆ¸gõá¡}0#ÁRŞsák¹êq¯òáJ„ŒÆjJõõÿ‡Ò¨šáa5ZÍt¦£œOä¤ĞOÜëlØdªÖÙìyÎ)Y 5§È¤ òt\àØ× ¾€¤T)ú¤ó¡uş›R©lÇ=ç"úˆÃ”Ú}#3}ç³Õc3ôBO†<iûî„ª·7»¬„~Ç_pÈ²¨A»bï©”º¬t3]Lˆ};ó©¨¨û`š_±lÀ³°Úè@‰àDÛÑU¶zºeŒyÜ Ñ11ÀÇ¯AÃúPŞ`?ÏJ#=}[_kïèoÇÙnl°zš™Šé½]4·HçkÇ}GÕ9£¡/›¹ÓŠ@¡ûê§{×Š"ıÑvşÜ8Ï9Öú³¯
dq÷–Œ€éŒ†›¡D{¬aL2¦V–ó~ØQ½ºöš¼ÌKo;ĞL@¸|¹¦>”¨[ÁÊ±nVF;‹Ú@ÃfÁíïÒ=»OV©œ¤øiå#¬°ı*OÊjfM£×î¸t´r¥a`Šó·cF›õ£$Ä>ê&J³ÊğCoWô–,ª8Rh(&„'ô·£Kûã¼i·´VmÇ¤µşü†”2ãghf7ù «òÈô“ÖmÌ©¤Ãìñ,IÜìñ@àf?k&şf3ªX”†ƒGE´—û"ÚÊFı«Ù‚YõéÏ¤Ú©:ÚëV½?qzJk×e¢7º‡àR¥}&U“·ãÑŠñ_–;¬Œ´…>U6Y’rRÔ	cŞàJá‚e‹Õ“ØÜU9Ù™#-å]"Šıªº7ÍûÕ cI€;ªP–*T»°B3Á#tú((°5¤ZP×KÍ’•ÙÎÒ5RË u®,ãÑ¥˜åå¨—ˆ¦¹íÊÜhzı”]¥£Mh9–Nlár>©ğní–!`Re×Uûînpjíkğa}o‰#ëjÓ#jh"¯ZğÊ‚>ÜÇ«!1í~Üœã8(¯@rÌlX(qVâ{Õƒ5'Í†ì3æ»we¢ÚöÀÍŒ¯Á>:>äW‘íŒıs.®‰ı¦…Uì¯zĞı¦ì3ö;}™•‰jÁşRÆŸö«ÈvÆşmRìàö š†…Uì¯z°~‡RÏ¨ïôş¡t^Ü+,ôœì²ıV©~¦7`¦N»¥botlÌ„JNÆòwGÂÊM	ø?À-Öêˆ{xœÕ•ßKQÇÙ4kGÅM3‹®KÄL¬»–JQI¸şX´PÓ24¢n3Çõâxg»÷®?Šğ¥ê¡åQïÑKÔ›Š/Bıİ;3ænn©¬½ÌÌ½3ßó=çsîp>dÖ­gf0æ1^x{£6NA¤f…ÈÅù*‚
S“²*Ò_ïá¼˜X²!'ˆW€‹QŸ©üùÒFf
Î÷ [,%Çá^¸0­äf=6/{kš®¦RxÎ£’ÚĞ\LhY¸°Hû&'y>D0Šİ	`Àü/,#e òŒj•}h|\İ—t%3 Ìx–a*n‹åÄ­[FTeÔæÍ¡JSÎVŞ©>.1Lc'Ì»ÔGqIÀ¢'t»ÄA¾
Œä›êÌü¶eeA¨çÀ¦ÏsÀTo­­ÕkÍpxbtÄ,JoôrQâZï–uÍ8£ÓèU]ñ¹u'tè°„~áY>ƒ¢®¤	uL¬ß\ğ·Ûzt@ŸPè[Òd*cU-1-"4«6MKç(×ZÉö«é7‚ÆIÍÈOGkt¡òiìÒ•"F×)³§¯R0(•B6Ã‚^ ®Ó²õ’îÇİ»‚‡mŸ˜xT¾?hiˆ+şÖæ|ÑcÎ²KnÄô~…ø3ºüÚ“cQÔ¹mˆ¿…õ…ñ¤¬kµg6±ö¹Ô•£n»|/ù–uØ€]¿õÈêşõ‘–ßŒs°Í4´k¶ò]CËİŠñÃŒ*nöš7´ìõ¡}o8qvGêÿƒàëÆ“r½ñÔÍŠé©ÙÄ<»<¶î¦xÇŸeåyMí%¯ f‚Ø.Hrøù.ÁÂ Üjó'«x…“Ğöÿ?4Ô”'[ÅT#ñå‘G+*ŸÁ`/ÏˆXF0Wu%Ph‹ÕL¤À°&<œã,l¥@;ö,-É%rº¥öÁ6Pç„?xœ{¬y•¿ 19;1=U!±¨$39'u"¯ôÄ6vo»µ4”©Ÿ’Ÿ›˜™§ÄÅ	p)ÉŸ¸×T.]’›ìÏh3cò$FC8û£†L?Ø<½ĞâTçÄâTM˜&s1…ñ:¥&–¤:BE
4$–ê¡JpÛÂ$]+’SJ2óóô`’•|šhf	ğÈÊa(GUòMb2ŸŠ¨Æd.	6…D 4'oàrá‚krÕr Ôt‚µ(xœ‘ÏjÃ0ÆÏñSˆFRšä²Sa§m§­të¸›˜&–Qä•2úî‹ÓôÁnFŸ~ß'KNª½¬4HÏõ³[¢eÂ¦Ñ$„iC"¢¸2\ûm®°-¹í¸§
­j,¾c!"ø3¶ÒX¸ï-ÍŞdµ¤£,MQaÖE˜±n]#YÆ²&+›"àÚ²Q’Ú¢œb‘
ÁG§a¾o„É+†©ë”pO>pbç­‚w}8;$ÿâR˜}iödáá\éwÙ¸½ç":]ºX¤°Ö•éB¶†YØXşJè]¼Ã°x}®%ñ°‡8QÈ	ï|õñùÕ×i4‰ç@ù-4¿˜§“~UK[éÌÉ®; •Sl9È«QÂ;¤
ùOøeÿ‚½+ûËf¾‡mõŞòfTÓ°´~k¿r)ßjä@xœkeİÀ,^˜œ˜ª‘š˜S’áœ‘šœíQRR0QJ‡Ih¢·5„ŸæOÑE‘>fŒÂıW§ªçî¢¡¤VÒQ(ÒKÎÏ+)ÊÏÉI-Ò+Óäªå …/©âcxœÛÀ¼Y¶ 19;1=U!±¨$39'Õ£¤¤À9?¯¤(?''µh¢¸;Tb¢Ÿ5œ9UÎ<kcNæflĞğÑPÒ‡
)é(é%ÃMÓs.JM,Iu„HjrÕr =7.Ø¦xœ31 …äü¼âÒÜÔ"†Ú—ˆ7ù¯úN©XúÎ^pUû„	XIAQ~Ji2P‰¨€÷É£3…ÚÎsù,®6ZºRàq ­ùÕ¬xœ340031QHÎÏ+.ÍM-ÒKÏg¨«=a¿<‘i{™ZÃêuÁ­®~†eåùEÙEåM·„v¼®^=§=ëeŠÉ“YOBEí‡»_xœ¥TQo›0~Æ¿ÂCÚmb¦=¦ÍË’n«&õ!™4i/“k± “4«òßw6¦´Q«í	|÷İwÇ}Ÿ©)+hÎ1ÕF°’§YAJ6mÅ5B¢ª•68BAÈ”4üÁ„»T‡¹0›ö0U%©(ÄtCõ¦"ÉÕ´L«©áU]RÃZÒ2ñIê(BÃ¾ç)ÿ¥Ê&qè„y8”¿VPª<p§i];¤»ÑZé•iù†¹|MâñMÂmõo<’®8S[®÷ßş•W{‚çÔk.Í7®üH#dö5Çı®qctË~DÓ4½ÙBÅŠÓr#IE„²V2|Çw}*:ƒñÈZdäFÛSsÓj‰?ôã<>b†õá©eÄğEñª•=W1ó€½¥É¢{Æ¶C®0#VœàOñÎä)åï”. ß][ap¿k¡ñl>nÒN¥€
RÁçÚn‘+D†ÁI¶Ì‚TÃ£øÊEŞÍ±¥ÃÇÉ/Z“u›SÅÄ™7‹BgCÌ RÈÜÏÜ¯úIÚ0–
ˆşÛ­Ì­­´<½0¹Vm=Ãï·áÄ¨d&rÀµáÛ¥eë¶° [Ù¨hÁ#f÷àÂ¸OÛ”7$ùÜŠ2¼ğï	FËúéÀÑ‘2 ä%û“Ñi€:¹ää<@¯·Î#Ï`­ÂÎıñ
×óÜ¸¼tÊ€¥º0ù¡ºá¿Xi±@= o9*‹nxÉ»0Úp|= Y*	¢Ï¬Îİ8¦‹pé×»9‘ÁÏE$¿>xœ’İnÔ0…¯×O1D*JPÖ¹/ì
T 
J	qã:“¬ÙÄcg·¥ÚwÇ?©²RBJœøÈóyæÌŒBîD‡ È)Ùã¥hw¢6ÚNcj9ÈÙ*“F;¼u™ÿE-M£tWı´FgŒ­æà7Î@Ö)·n¸4CÕ¨Zoİ‰FUYJ’Y;Æ^8¬”’}5‡W3ÿ7ÂogLo«ŞtÒ<Çp²`¬´„\Â9×V€$ôY¼İ£vßí|•‡øSo…–*Y3Š¿ºé‘.í­İD<—îfx¾¥wµ³À9¥¶Bâı±€üdWŸ§­l‘-	§ù¼Zƒ£	½zÌ¿¶†Ò™Áv‘çü¤ª+Ï¹î•ßğtrû­õİÉÆJµ1òÙ´êî¡(/DlĞ,¬Éxş]ŒüóÔ	Êş^·¦ÍcXG’•@(î±!İÂ3£’Õºí”Ñ•i[‹ÎöU|ÎáÌÂÆ/?tVF¤¯Œ_‡ eûé!z‘>FLÚ[G~.ó _â]ñHü*ú	“\ÄšD˜YoœÆC¾L1¯£‹¯“p…¿&´Îë§ù 0şü‹Ù­è—Jx¸ÅËÇæ†	Q:¶qvuÁı£yµåæîÙĞ¾üuOİğdûâ{dGöU @Í§xœ340031Q((ÊO)MN-ÒKÏgèì^°>>ËõóéI>wÔƒüâÛóê< ¥w´#xœu‘1OÃ0…çúWœ2 ¤"öÎZ˜¨³ë\S+ql/´¥êÇiH%˜lŸß÷î.hÓèA[Óâ«Ş7zK¾ê’ÖO¹XeÆwŒ'ÎDº×–ıNïTÄÚaÇÖ«føZÖ~PLnÏŞiÛÁR_ÙÆ–Mg]YUûÒYC¾dt¡ÕŒÊ&uºU“ƒªn™X5Ëdÿ[~±÷mÃ¨0É3QÁç€0W 2õ†á"V†0‘?È&2¬ï0r¬Š«û¾3ğÇù)?ş­-à®wy7ĞGÈ=uğ0'¹,ùOp¼şĞò ëYUÀ¶ßµ667õËgšynøÓZäf<ÁaŒi¡¤”·€òm¬€D~™!È%[nZ›\Ç>¦?q`üz&Ë"ÅûmPÉÂ­xœ340031QH,*ÉLÎIOÉÏMÌÌÓKÏgXÿ³é_ï»^­ÖÌ1ù2&…‡ä t‰\¡xœ340031QH.JM,IO,*ÉLÎIO)É×KÏgx³÷ã·íËEÂU§Ç|?-ı’oÑ±íõ Î£”²*xœ•QËNÃ0<Û_±Ê%(MŠP/Üâ *çgI‰ü@¢ˆÇnBÓT­*öàÃÎxgw¦cüÕL[Á,¬¢T´ÒbJ>Y#*f•†¨vå^2®Ú¼V3µ^«<<³"”Œ(™²Tİ`îœ¨"šPj¿:„Ìâu/öˆõš`¬vÜÂ7%¬EÊw…¬¡|3J^EÒ#QII†kÑÉ=FåÏø¡ôÕI1gaöùÑûİqBğHœ jí¯öûh´NKØ:‘ı}zÚ¬Ük¤”Œ^e·›Ê»GÈÙÍÂM²Ã	kÕ^ûemWñe
‹y°äÄè3ş¥°Háb>J$Á´C	™NIƒÓˆî
ØVˆ7[.}kÈÀ§]LÑ×é GÒ6Ë_K¬Ùh xœ340031QH,*ÉLÎIO­HN-(ÉÌÏÓKÏg˜#¤³ÁR¸[ñï£MJ_îï³ı9 ™*u¶-xœ’ÍNÃ0„ÏñS˜œ©$w¤hU	$à€T®•±·éª]ìuùSß8.%¤Bô+c³‘·BnDÜŠ@ëù›„-¡5Œa³µøˆeàœuwè‰ç5Ò:<—Ò6•Â^®…{
«²VûJZãIª:$}—º%s–ÉàÉ6ó(ùŸe÷“î± Ôçòi~ˆDÎ
ÆVÁH>s œ•ˆqÛÜ£ö4ïˆ"-ü“e;s~5á?ÃËÅa”ù‚e¸ŠÀÅ„ÔÑ!s@Á™VdÙµ¿á£uì·¼5Î=?*ƒYìÛ®ßfù ¯ƒs£Á˜òŞ×c>gVµùvP°ı¡œ®–)…¦ôªı­s®~í¥†ã“òeŸş3ÉT¨Gx	à)%9™›²œÊ)M[{ŒóÌtÎá=xœ»Æúˆ•¿ 19;1=U!±¨$39'u"ïKvGsão!F(Û)3/%3/İµ"YCS!µ¨(¿H¡š‹ªg².S1L?L!H‰oqºLbò&) aD)‘§xœ340031QÈÊOÒKÏg˜Ö)½ÕdwccÁ6†ºó/)]Üï¸Ù¢¢<¿(;µ¤¨çœcó™»Œşò¿Şíz"{õş­½ Û±´Mxœ¥“ÁŠÛ0†ÏÖS†9$¥·@Û´¥)¥‡Ma‹bË²G2ŠœlvÙwïH³ÙRh	X’óÏ7ÿŒÆ(·BI½o¾Ù5!z×Yç’,/­ñòÁç÷Ê²~-³NñGÑï´oú5+í;»®µâ¥³†Ş‡m ~²;¡Œ••ŞêY#ÜITš+;Ûišy¹ëZá%×˜ÒÑò.×¥ğ¡U$å—Y/XŞÚvÏNtt¨L»ÏÎY×SµÒıÙÉe4Oú=—!ú>ÏĞ[YÚƒt§¯ÿÊu	p^aÕÿŞÇğ3–dáRğjÿŠ
ºœ„øS'aƒ{ïúÒÃÓÀ €IXÙ$k­Rhp‚ÓÀ¾Ç=y&¤îM	?äÒkE£±`Á²<"Ì?@Äc4›;ô»hP‰Ã°ÕVwËzåuÛŞöÆh£hª.D9hQLIV$sÒ÷ÎÀ[,å)ÈæRMa°6Oëóoçt”°òÂyäîié }	l1¬E°½a|ãpB[ªEæT6lğ´¸F_…½Š?Z·ÅæaSÒ³½n«¡ãÜwQHCÑ¯»8T/f˜½8”ãOˆÅÃYƒó’ÅËj
÷Áoªÿ¦ª¾`İ4Ÿğw0~y„FÃì§œGva
F·E"ÆU0ß+Å±¥©mMóT3„Ñ‰–ÕŞò)$¡ß¿ ª
«Öêƒxœ»Ây…“¿ 19;1=U!±¨$39'u"/;ŒgÃam\~’&º¹”)	 ƒ9»µxœMAÃ E×ñ),VP¥äÕH£Î\`Ö„JBqäM;Uî> fÑí/}û}/ÆÎÆ;4œƒî›z€p]ˆ3Jh„¥”İ=(³ù²öÚÒµÂÃ3„î/Å[÷ËfYP ãš,Ê	õ
#ùÓëüñìX*ÜÍúlÒVÿvyå„u]Ú|Ç¯?^½-1ıµÖ¡huÏM¡|S-:fbU¯5“.h_0_i$)ök,Á„*–˜Blkfƒş€.]²«xœ340031QH,*ÉLÎI/J-È×KÏgØ9Å¡e²«ïaõuŸ&~Ï6Ú¯ E\È½Bxœ¥“ßo›0ÇŸñ_qEU1Z+å¡m¢)ÒÄ¶$İs™1Ô*ØÔj²*ÿûÎ„‹ÖM“ö„Ï¾ûŞ}¾6u*ÓBBjQ‰Rndm…ÆSUm,BÀ<_r>-óŠ>Ìëó—¦J•¿PøÔ~çÂTq¦Õü)µ‡4Sqaæ•ÖÌQVu™¢Œ)Y–q¯g„?I¢ù?=4nÎ?)ü@cÊ&&L,¬l|2†‡Z‚Ñ¡AÛ
„7æi0VüK¿`GÆòVHäëd[@Véw²C8sŒO®•ØZWÓoƒÂ8ÅãØ-°5Ì¦¼î­$nOêtY÷Ğß¿?}#æI
0›Læg…ùÒÊi?b!Ékj£w´ÖØĞP1‘Ü,àqlW›¬“İç¹@§•Œ “°ªFetßn?=¬¶\~ˆàò:„Íj÷°IÖÉGPY¿<2gSÓ–Øõu½lÍGŸ·/å~yÇ¿º9zê€œˆ ›ŒJ:~t²}°üeæ©¼Ó½X€V¥cn…ÂèÙó•ÃÍ¿ÃE.Ÿ.LzAÂØÌ'­ãô‡¸Aµ|şÁP*ÌIøDÉ‡Ğ™ë¹¹ÃşV¤:¸êåøzÁœèÆğœïÀ3B:£­ãiøş WŠ\½ÀŸš/qÛªxœ31 …´ÌŠ’Ò¢Ôb†İ³ÏdNœV!òM²Á}«õÏ—~&`%™y%©éE‰%™ùyÅ:¿g},u}Zl0©ÇìØ¹}•,±Ñ ?}¨ªxœ340031QH,*ÉLÎIÏÌ+IM/J,ÉÌÏ‹OË¬()-JÕKÏgèü#¼"å˜ÛMõC]¤wW•  qQi¾Ïxœ­VKoã6>K¿‚5Ğ‚
ª)=¸Èa#'©±İEg»À^F¢e"²(PTât‘ÿŞR”,?R4ˆ–Ä™ùæÁá7¬xúÀsAoÌêJnL£EÊu¥´!4F©*Ø˜¼®¹Yá³öÓÈµ…aÀµ‘i!ş>#£\šUsÏRµ3ù OW\?óLÆ•VFİ7ËS#ÖUÁˆsuº–©VıJ?!x®T^–«‚—9S:s]¥Ç%qªE&J#yQÇ²¬E
‰¼¢nDmbˆÒ+1‰šCârcæB?ÊT|­EÂkq<¥ı JB—¼ˆÖ¾µC‹à  [èkáÿ@Ë²…FcÌ2åFª2ÎD!…~öÅ±àS½+ø
 =ø'¾|à7ZeM*ô»zy@dìíıİŠJ½‡8µ4J?{ä7ìğ1ğ~w}\h™å¯`ÿc”‚Fõê‹{« 2óÆeùZq[cY.5_¤^ß6ûë…ÊsTŒÂÌjC.¾^Ígß/É99ûõ·äÄ>ÂĞ<W‚Ì Ù\Ûïà°´Ü@j£›Ôap'¸ª§’t¿eS¦4
ƒÄlÈğ×²KÜTx™Šâ Š•\TÌ0Å¾"ğ;éÄfİk|tì+)$ìéh‰µ²öd;qø†.ù"gJ#BO‹ÆDh­t„UÈÏ
ˆLÎ	2"û¢hÄ>fµŸÓÆ™SdOö™o ò÷T)5›1I]ÀØçÿöpÚ‚R¿x4kÕ”ÆÄû„}ºıGŒƒÅùñm•‰íÜ0¾w¹¥(±0f	jB"³äFÕ&×¢¶X`ûr™®mU-)ÀÖ2ÁKëâ§sRÊ«¸<Qh5,Qb	Ø
<8îÌÀšÄ:<Yèêr°b#‹çZìP>8;2Ğëü°ˆvQ³NkÕj÷‡s‚“}>E|ÿNeÊ¬ğ›–' ôÖY#;¢Ñm'B3¿a`Óô9m“ÚøÀ{Ø1Ù­Å˜¢Æ~‹c‚ó [Šaà´A#ˆ~å­¦bÉ›Â|–YVˆ'nû&ĞäV5ÈueuÏøZ\™-á†A7]C¿Wô æÄvç‰F-6¶¯VE±UY´À<{m«mJ[=âDìVä²ö|[œ·a€ÿ<ã¾õÜÓ’ÇÒ“KÓöíãRN­·úE$‚a [!kŒ£½Ù°¿ĞCI[÷¹jYÚö´¸ùOÌ¾Ppı±{Ê7[Øw^±y“sT¿â†K:röp‚¡‘3òüæøsB~~Ù“ˆ‡Xê…b
yÇáZO ¸Ì¦p¿óT…g(#H/¥ønÕC[5´€àmÒ ¾;‰ çq¤É2ºÖùlù½cHÛº·Lò|İi^ÖxeNú[(õ·PÛ
[ëÚ¾‰ªøŞƒºô~::,(–¸EÿåğxÃPüpŸl·Ëv€A7NÜç‚¸-ê±¤PÀ/V:àtWA,àp¾OpÀÃ¤Ü &»W·×îz0º¨P¸w˜½šŞËW€½Éoyåˆ!xœû'Ù-É_˜œ˜ªXT’™œ“º‘7…‘Êv/*HÌÈl¦áê§¤æd–¥Uê§%”¸8¡Ê<JJ
@ÊäĞ•e %Ê&09³A™LÒì0áLVÒPE©ùÅ™%ùE•HÚ~1ÙBµmva®eRËLÖQÈÌK+JtÉ/ÏÓQH-*R°²UÈtÎÏ+IÌÌK-ÒóK-÷t|’ÍrrÛg¸5ì–pö>v8û=û38û‡,œı‡#%îqHn.äìe ƒŒoş¢xœ340031QH.JM,IO,*ÉLÎI/I-.ÑKÏg¸¸i•ó„Ë¶{e\$ûjÎù¥ É_‰¶³xœíXKoÛ8>[¿‚«ÃB*	ö ‡Äö&n7ˆ½½ni,s+“*EÙq‹ü÷¡(ùmo@E.DÍ|3œùæá<ùÈ3`\›Dä02ÍPr¥ñ<1+”6,ğ:~¢ğã“ññd¢R!³øŸRI:`â©1Åú³ıc…K£Q¾¤G:Âgßó:hU$9¼ÍüL˜iõ%j§â£8Ÿr½ä©ˆ­Œz¬&çfEÎÄ™:Ÿ‰D«Õ‰ƒ‰ç¯	)ç¥ÁÆLU<ÿcÍfÏ¨ÃFwM¼»–<om¥Fù-ØŸâÉT¾ÂRÆ“‰•é"ék­ôaØÏF©¼Œ¤b’G5Šú×¨ÕiÛŒfL2ÕÖ)1YÆ¼,Aÿ„TY	VH©,‡(S9—Y¤tf‹‘4t³ĞóÌ² Fj#RaˆT%†}ñ:"²§^Çƒ½Ús´ÅRwì={Ş¤’	,{Õâ‡l¦*ìs’‡|Æ0ìâ’mÂb¿ ô:bbµ~»dRäÖ©£Ù¨×Æ£qZp”G·Üµšû\²Éi—ÇÀuO-äš×(Q#€>…¡Ç$²œTyW’ï“qUß¸NÌEÁUxt%“iUÉ±ñ«Ur
ğ©BHÿ½-ß¨»G„"3ä3¸`ş5•ş¾÷ Lğı^+Öƒ9äª M(FÈúBÉr•”Íë:pò½›fÓh€w8cû\ıÆ”9‰¡2C‘¯‹4~Fƒ4lÅúŸ*¾!åî¼’¦P“ßŠÉJ‘bv2Å„şç"µŒÅkı´LŸÈsçïŸß#É²ùl{)YmÛjtÏu	ı§ºßÔÇAM€ƒxNûH"m“Ã5§l\é¬šá]ZÅèÌÈpS•A¸éb:¸åK¤Ğüš—–&ãìŸ¤ÅÿÿA‡Â¡Á ‚'§iàÁ[ôÉwY|3z7¤ˆ|øR“ëÂf¤Zøgµ¡ÿ-nlLM˜™“¸Nıç¶g·¤h¶=.%Ew`¦*½W%^ßy!pOk¶ìsn3¬Õx
:Xó‹‚ãlD·ök„£< uÎ½SÌ0²c\(0Ít~7¸ë_E.K~³6^v}M”&³áö€È‰æ.'h¸àÃè9Ø„ĞÉ@â†)id?ÜÇ÷Áj”érî0Å©&Ã»·k}¿‹²UÈiWE¯%,‚Õöº·ôğ¼w-sw&­ìÑ_r†¬Ÿ¢#­¹k•.£ë%†ÅÒ—Œ†áÆ¬Ü;âOj…fÈVÙåQ£JB‹‚RVOào›x_Ãòz¦·İ˜{0áUnîDšæ°àš¸ñó«äš§.{ªÅıüiÂ°§ÅßÖÇ+æ?@‰—D×¨¶Êg»ñ;Ìã…qÀW§»Õû5ÿ5=;À(’{Àw‚_±<^êãW©·a}4åAµğP¹ßÒ¦.ä]4¶ºÿB¸»*¡&•¨ Íé³÷/
sd®xœ340031QH,*ÉLÎI/-NMN,NÕKÏgØdgf—öfÒ²şóób'øÔ*­•ã z‹‡¸^xœ­TQOÛ0~Å­S‚Úä½“ô	1Œ½ ©rkê5±ƒíĞVÿ}çÄ…„R¤iËK’óİçïûîìš‹5/¸qR”xoñœ[dLVµ6b„V·nDŸ¨„Î¥*²_V«£H!İªY¤BW™Å¢Bå¤ÎÖ|¹æ“BûË«ºÄÙÖİ¡y’/tÅ¥‚~a.×r²âfÇs™zRIaôÄ!r‡íFñ2ë æô?·X–·hD-ğÿ{t©z@8„túßğù0æv5BÓùÖ™F8øÍ"ƒµ¶Òi³ƒÁ3”Ş¾d±¨u÷Æè¼hV\ö³Ûp¿'òn{Ò»÷ÓÙ3cËF	ø†›Š±ÿ/ã·ú?P=fÉ›åı­ï®1
>‡P¤§c:ì…éëîßÏ\7Ih~÷übqÜ8	ì87HÓuÖ©‰…ÛB8–éy÷ËG8yÚtPr‹ZGñâ²l­•õc@c´I¼W!½Áô‘¾Z’Pk©$,’Ë¶àÓ)(YBçpk:ı¶X^.‹²¾__\ÃBÁé‰WÜØ/ç3cæ³­ÀšnÅ"ûœí9Í=#ICvè&6J+n¡Ğ°ÆİF›¸ÊıÂ¡æÖ‚[¡çìÚû
tì%¹JµY–@§º’Š„_Ò÷Kz)—(v´]Ê¢9´Şæ ½i¥´«Î¥Ùİñ¾u_éò-ŒnT'ãn|Ò+´–.dïÖ%¶ƒøğs±s‚âQâGë/¤¾ä¦—¬ë÷Œfê$|“¢xœ31 …äü¼´ÌôÒ¢Ä’ü"†WK~%:‘¡]ôQrÖ¾Å3*sMÀÊRRs2ËR‹*fI¹5mÎ¼S¥Ï<£®È¡7WÓöş"¨’üÜÄÌ<†ò¼M«™×WÙTÏ-/¦pj¾|'TAI>³ë–ÿ'ş\;Ş-}±ÖÓ+a¿ïˆljErjAIf~ƒrUò!‘ß_®:ßèÙx Ñjó¦ˆš¬ü$†GÏ–ÍM÷½¤?5 Ü…YVòçˆlQjA~q&Ğ•—”}©O=ı‹/FÊTæ»Ô¬œ<~_ˆ¢’Ôâ’b[ûdû/“ÖŞkºª¼í›œŸ×4{)ˆ|iqjrbq*ÃË½ùû¢\.İÚ°~ù•æú7M Ÿ2~ xœ340031QH,-ÉˆOÎÏKËL/-J,É/ÒKÏgH.>ûìUF{&£×qÅ}ÇæÌWı³ ª®xœ31 …ô¢‚dã®s'#Ã'…\\V—}VŒEÁ,QRRÀ°­ìû¶kØväZ÷Ø÷T}¿í4ˆtvbZv"Ã‚‰Æáâ•>·J¼&Şk¸²§æ< A,$Ì£xœ340031QH,-ÉˆO/*HOÎÏ+)ÊÏÉI-ÒKÏgè—­P¼æºÑÛ³ßÈ²rÍÂk‡ÍUÏÔN¿¥xœİWKOã0>7¿ÂË%¨M´ì¶°¨Ò¾Äëº2Î´µšÆÁqxlÅß±¤)©K*Ğ"öÒØãy~3Ÿ“f”Íèˆ …šÊŒEª¤HÇç™Šø^o‡¡îÕ.'\M‹ë‰yóL©| 1&b0çLŠ‚y–PùmŒ÷c1§<%[ØsŒ)SšDÆ>ŠƒÊİwCòboÑ\»©STâ%•ØŸl6‰$ä™Hs¨RØw'I¡Äu1 d`ãîÿÎAŞ‚4^K£Û;^àyê!Âê®’\É‚)²ğzECš—½·İ	/­Ì{ô¼q‘2òî–álr°"\"G‚*dJv—I °Š~@
Ö÷zuDŸ‘½¥f@¾Ğœ³#ô?L8¤Êgê”Óí³O˜9÷Iãé*y:	ˆ¿·–ĞzèR ç…çõ¢ˆhß(æ[BpY:ózl©N	Ë”ÃÕlÚÑ¯ÇÇÆìÃ!Iy¢A¨PÀmŸTMO¤¥·4áñ°ôñS’º*4vâô‹æùñ©¤.”$Ü‹Ç~¸¢~7äª†«ulÓm ×£š+ˆJ
w~ÅµNQ„§ ¾rHâüŠ&ø˜ˆäÎLZµ¥İ3‘ÁE£Ã…W(ìR¥Œ2?øì zwS%‹GS‹¹Õç5#gó*›7wçÏ¶¼{&”™³5#·Ú]ô­sÂ‡Á¥†²ÔhVh½»†¾-<xO¼FşĞñcçbé!eÕaCªS5‹¼½Pe©TKµÊÉ}Æ‘K£Ô¨àuüiß¯ë³@k ŒVÓYËŒQwZfT7¥úÚL@=0Bò?Tq‘àL\o×¦¤K¯#77†q‘TëÙa?ƒádêRòU.¸iÚŠØ‰¯›Ë|ân‘’›ÁñxBexµibşBîv.aKÃÿ•ÌvD†bıi@“¼#™×ÛµÉìÒëHæa\d~æ­ÚòÙ‰®›yºn‘’›®FŞâQË:p¥?æ´ÀüÏÑåÀĞ.²,´ËŸ:§Ğş6Ü}ãcP¼ñÊ.›óºåmgù¹¬Kvq¹©İ‘Æ-“6ƒ×¨t$oÓòÊ®ÚåÓ¸iĞ‰¿Î2åùO©Û-7k×ö“—ì“ñyÅOä.ùw·yÏ„´/×¿®F©êw…~xœ•”ÍnÓ@ÇeaZ; Â¥PL…ŠÁ¡'#¤V­ZE´
¤8 ­3u,Òlâ]S$Ô‡{áàRG<â	\¹ ¼ñGºÛ	¾Ø;óŸ™İù÷Ç­÷7´>r^!Œ¡^®- €v G=Q÷†²åÏ®da•õK(8cÙ_®±ŠyWTÖ_¯.—ôe×£àÀrğQİÅØíB=¼ö§ïÊOEk‚ë
>Cóå‡RR+q6a ¡UÃ¼“'aßæ«j‹€ÿN[½Y_]Q›¸lM«D–¯êÒ•Üø_ê¢ò’İ»ü¢PòÀ5|ß°Ø@RYÒu‡¾©…/´IqµÄN/V
³²½9ÍÉ?<éã·a>~Œ:ÛHöa%&^×6ø“º¹)tŒöËNÏ]]Øè O!ÇØoó¾oË¥EIA÷¥\!ƒçÙ±Ö6Ğ-ºm"ú÷Q7 Ó‡Õ
G¢fğ¯Ä­öà86°u­|~Î>«·—¦ì%å9ƒĞ>Ä¾‹)7epõ	]cÌvZ…¿zegZ‹ÇŒŸRDbã=Y‘IÄ¸•ºs1Š’ŒR®c#;vŒQô`äÔæ"5û¢®LôJªmRXŒMÔÏ‚Mª…MnËckõÛˆBòÓfb%Ø¤\[vì›èÏÂûØïe-MŞœİ’ªçƒ›ŞFåbd|N‹OÀäp¬V«±Éıú‰vxŠ³ĞE‰t[Ïğ.nC×ä«Ñ(¤÷gÎLLÖä—õıiÌâ¹ˆÀ—NJÿ —»·íïƒUxœ›!ÿ[~‚!WbiI†K~nbfŞäFF©‰k˜•¸8'vÅ°ƒ%Jò!¢Ú“McÒA¨×-NuN,NÕT˜Á4{ó¦TÆÍQ,oÅK3S¬@ (5=³¸$µH/4ÔÓEg²3›š¨'X
.”Ÿ“êé29—Siò<¶	›õÙ?0mVàºÏ´‰×ƒQ”‹““3¸$±¤´ØJ!-1§8U(2Ùƒ¯‡ÌN¾Ù8døØT&Ûò‹Ks!$KŠJArµ:
©EE\µ\ JˆS“¢xœ340031QH,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgĞë3’Û½aƒÂuŸØí{'&ı½•+`ˆ®¥(¿´¢œ½¬ıùÏÇ|9¿Ï¼gmø^Ó¢ H÷(RàŞxxœÛè~E£ 19;1=Ua"§ÜÄh¾ÄÒ’ŒÔ¼’ÌäÄ’Ìü¼‰<ÑEŞù¢‰lv`´fÜ¼Oô3/ æ
‹¦xœ31 …äü¼âÒÜÔ"†µÇ&Ww	^¹s^S$fÙr9‹I÷MÀJ
ŠòSJ“J–í4VôÛy=½Œá÷ißïZİ¿¶ ¡M˜¬xœ340031QHÎÏ+.ÍM-ÒKÏgÈÕL\1ElÿùIŠÛ~{¿º·Cô©!DYy~Q6DÑ¢úıjf=6?Ì¸²[¹
Î–‡  ]ƒ¾êÊ=xœûÍı››§ 19;1=U!±´$c"¿*k>˜amÃ¤RóJ2“K2óó6Ö
1A¤6OcJe ‚RQéÆ#xœ{Ïş§ 19;1=U!±´$c"¿k>˜ácÉ¤RóJ2“K2óó6v70A¤&1ß n‚§xœ340031Q((ÊO)MN-ÒKÏgZ#™8í’şFÍY«Cİ²¾Í  •ÀèÂ]xœÛÂ²……§ 19;1=U!±´$c"¿;k>˜kÃ¤RóJ2“K2óó&Îß‘™Éx Hã¯ªxœ340031QH,-ÉˆOÉÏMÌÌÓKÏg8ÁX2ß+sòµ³üRŞ,3”Şş5’51 …Üü”Ô†ĞW–2›2¾äÉ{öãİ²ücoßY ï?Õá€÷4xœ›,LaÃf¢Ô´¢ÔâŒüìÔ¼É	Š“XÕ6ßcşÇÄÃé^”˜Wâ˜œœZ\<¹€Uz›æäìü“]Ùµ&ßa3š\Î*ÚìÊîÇ µ›èà[xœ;¦pQaCûäPVÕÍUìg8 7ô	ã"xœ»¨pF}C+;'gPjzfqIjÑäÿ¬b¾)%ù
Zù‰¥%.%ùz¡Å©EA©…¥©Å%@®¦‚ºTqA~^qªBjQQ~‘&§sFb^zj@bqqy~Q
ÈÄ8TQ@ÍF1]ºnùEéù%ÈVÄ¢Zª ›è*Ğ­-HI,Iù0/1wòD6ñÉ“9„'?à•ŸÜË¾nòd6ÑÉXY';pØM6çÒš¬Ç®4y+G$-:YÅp³=gãd3 g%—Õä;œN“¸y'õÈrÛOŞÎ4y%·öämì3X'Ûs²MâQ©Pæy
6ƒ×r23P…#¯˜Ù¾€WÎ~À+Îu$r8°”–f¦LäŞü‘·”)bıùêÑ¢,†–@±ÉÛyùY4’K*ò3 J¼òäAxœ;£>McÂ!¶üÄÒ’£‰R†Eù%ùI¥iº©Éù¹©EÉ©ú¹øâÔ¢²Ô"0•	Õ/3œ¬ËèµYéßäre‹¢ÔB-ˆœ^@bqqy~QŠ{Qb^IPjaijq‰¦‚VéÉ™í¼5;Éü¢ÌªÄ’Ìü<çü”T¦`W7ù"»ğätfm#s2SóJœ‹RS€TfbN1#±«›üÅÒÙ¸ Ô´¢ÔâŒüìÔ<&a(Ù|Sğ3# T8‹kã€Gxœ›¦qC”£ 19;1=Ua"g´\biIFj^IfrbIf~~J~nbf~n~JjçÄ*t%ù@‰ÄR$±0Ã‰Riğ¡*,Àøkòa6e)°KI¾^hqjQPjaijq	«© ¡5™‰İ}r»'X,’Ã8ùŠÈöi“¸vmÀ»	hÜäAVK4è¥¦g—ÀÍiÄ®d2»È~4éÉìRbèZœ3óÒS'¿c—š<…]¤Í]‰[~Qz~I@bqqy~Q
në'?gwàn@hAJbI*(lòsSñ¸Uáde!ÓÍ3„êqù©oëxœ»!ºTlÃf~·Ô’äŒĞâÔ"§J99€E‚­Èğt™¼•Umó=æ_l lö~ 	xœ340031QHÎÉLÍ+ÑKÏg`PûÔ ¥°j©µ\Ä{Î<Éw(Båçææç­|ÿ<î#»Ø†)¾K>]p|o©æTQQ~N*H‰à‰Ï¯>ÉyÏè°ìòä‰EW/9aURZœZRÒsâÃ­]²^Uİ{«ê¿í·ÚY ã€=®¹xœuÎ»Â0Ğ¹ş
+ ;kYC+fš¦V	Í£$ÎP!ş¨$<ùúêH¥dOèeâûÑ[©€¶£Œ¢“,[iŸF ğ4–F“cŒ’b|AQzk½ƒâLn“[íúolºö š>¢wËÚ@Q“
ÄÿEœû­)»Š:Hñµ:aşlwIÆÔ‹Ÿ]Xû[
zÓ?·Şğ‡ùO«à€!xœûÉøƒ‘£ 19;1=Ua"ç 7Xr¸%xœÅ‘=oÃ †gó+§¶ƒ³w‹â²´ñœ`¸:(†CøZUıïå+JZµsx÷'äYLÀQ:õh„¶ŒiãĞ¿cM;i:…±“hVâ4Ã*­ÚHHhÙ=côî€oĞ´|!$ñÖ<õ¼®t¡†tpTãc¯Y³ñ Ôšx*ÔíãV¹,ä (yƒSx¡êõ0Cñ®böT!Åû¬y^HwùŸ#mãğç=Áî0
·ÁòpYó¯àÁÊø\¢ÚNyÀ¹¤¿°ôr.õ–"çúY„ÄK„ßÌ_R^ÍÚĞöÍiË:•ıÙ:viéù¿Ä»á€}xœ»Ár…£ 19;1=Ua#§? 9D·½xœ]Œ1Â0EçúVĞîÌ]XİIS+54qÔ8BÜ´Ráééÿ÷½G(&ëÔ‰7 ØGYkÇ:å¡±â['âfjsæ±ĞG$<ËL˜tÉVñ	Õ±ÃÏ­ZÓ÷%ºÃ¡.¼%	]¡:O»Zpp+mj(Õ.o\ô’]8*Kø×Çoµ¯~£2¾°§¤ÆGxÁZ·Lã€xœ{Ë¸‘£ 19;1=Ua"§ÛÄ0•‰Ê<!™¹©Å%‰¹\µ\ ÜRö³xœm½Â0„çú)¬Œ°³Ò…!3u›¨’8äG!ŞZX¸É§ó}ç©»P¯)§SÍ–´ĞÖsH($%j)ªE¼î^á!ª1¦»„¨Vl-;¨vlÔºFÄáw¾ÉÆìSĞ®ÇF¶K†ğ¨¥Àsd÷³Í§†E³rAUğ¬*¥>éÉc8q¾~ m)ÆùwßáTüúğ/ºV»à€&xœkfjbâ(HLÎNLOU˜Èù )ª’­xœ340031QHÎHÌKO/H,..Ï/JÑKÏg8Êë¹ÏÒ¯Èh£¿^Õ¿¤XÙîBT§å¥ç— ¨–¿ÕqpF£(+/ƒcÉ’cA>Ê§¡ª³ÊKâKò³Só@êŠÎÍ>Ö4y‹CMê&½¹÷¯¬oÛ«	UW”šY\’ZŸR’Rzı–nsúÑÏÎ5Wò9[–³z-‰„*--HI,I/-N-ÊKÌM©V¼%t€¡àñ³şŸ’æ<³µºí
 *U”î€Hxœ áÿİİ›[hEşhƒÓB‡Î¹ÚŸ˜:kMğ³‘¯.å¤4¸Mxœ­SÁnÛ0=G_!äPØCbØ°C®):Ø€b]{®"«¶VYô$ªk;äß'*ucg)V`óÁ¾ÇÇGŠì¥º•æ2b{ŠÀ˜ézğÈ6Š}Z[>o¶qS)èDmnÍ²•şAÖF4°ìŒò°DİõV¢Æ¡öNZAÙÚ¡Q8QC'éÍÙìNZSK?ÑNrğø‚~Ë'JJNü)«EŒ¦>@¬ÜL	­Zwïç¬dzÍÏÀ7€ç2„Ÿàë¯úGÔy@ò_lvyùé”Ó—BÆ5üú{ ·šç×l6$ÀıS8Q¶ŒİD§xÑğ7G‹•ü£Æ3£m¦x¡ğ“ájix÷‰y\|zÑ;~r”ğÜÈ*w’d«Dë®¤ºØµR.eÈZRÛ!ÚöZÚéQÅï§£ûOÍeíI/C×Zÿy·eMZÓôú$YQ…dq¿ûÕ:ãc{€dm]—V‚Á"pF¶‡Bd{;u©dº½^íAOá”Q”\{Ÿnhoîù®ª!ë"ïø®•Ş3òËdï$£¹y¢Œ8dÂxg]ş%ı|ÿ/IŒÂŸµk°->,ø»·å _ÒLlèÁ=ºØ”ß ØáCåcüË¦òê‚bxœ»Áy{Â6çŒÄ¼ôÔ‰ï”'ó1*1)(LÖg”’çôK-H,..Ï/JQ(.)ÊÌKWHÈ*ÎÏ³RÊK-ŸlÇ(Õ8YI
Æ¬f”1ç2JÃ˜ûe'ß`Tı—QVÙh+…ä’
=·ü¢Ü°ÄœÒTˆá¼LÒX÷c’‘ÓQ )(@u˜¦‚VKCJK3St¸89L?HÅ1È†%k¹j¹¸ÒJó’4Òáv¼dZ¢
aÂô¥–¦—h* ı™’X’
U¹‰ùîäÌ2Ì@+&û°øÂ\6ŸÅ ³Û@¿@xœu’ÑkÛ0ÆŸ­¿âjX±ƒ—Â {ÈèÃHİFÇX²õ¡”TµÏ[2²œ,”üï»“’,YVCp8÷İï>©•ùBVFön~íŒªiuˆ(.Ó«RnŞ?sÓ\T¦–ºzû¼rü[^Æ"Â­[„©Y ×R5tÎö¹ƒ‘jø+Õ9´X„Ï"úÙ¡ı|ôRé
Ÿ;£GqOõ™*2Ó(‡MëÖñ£ˆÆµBíX~,Î}ä,šä†N;®³ÀãMôXà¸>ãHµ¢ìuj´Òá—»©ïKò°×Át˜[t_q½uL!	2@kMyo£+à ¾áê²	W&ªÒÔq‹nnŠ›É»Ë÷„Y©ˆÈ½·¼…Wb1ñ’û‡§µÃdO¦{ô_²V¡nß;ÙRıxp´Ñ?Ø¾°cÿ.m‡ôGÎç6/›˜$(`ÀÍş3Sš®A)sdÑßi‘*a–Yğ°°nHd˜NSºı8N?À©¹u—’V5ÍmÜğÛ–IÜküİbî°€.ôCãFğfgÛ97(´÷±¬«ø26boy’rÆSD´¡+ÏÈœÎÙ×<Ék ¥T5A8-gÄ*öĞĞMğË·Çp˜CÈtxtP´=IÎÏ·â‡;Ÿ +Ä«dJ/}¯÷‰ùı¥£]áƒxœëçèãà(HLÎNLOUØÈÉÆ 0æ¸ZxœµTMoÛ0=G¿‚È¡°‡Ä°u‡\t(°E·ôÚª6çhµEO–»¶CşûDÅv†5°‹äÓã{¤¨BÅ*EP•Ûm	¡ó‚¬ƒ@ÌˆcŸ)Áæ©v»ê>Š)—‰~ĞË²Ï*Ñ2¥e®cKK‡y‘)‡R‡Ö¨Lòm4NÇÊi22¡\i#sæ›‹Ù£Êt¢Ù·§£—’|,[ˆ¿ìñc¥ÊªÒÉ$“©ûÒyGãÉÇ÷s
á„m‰öTX:oJg«ØÁ/1ã„Q9rH›î¾—dVóªÏïÄìJ•åO²ÉR´a†\S†—€	Äúğ­fÄ^ˆo•‰!HáÍXKÑ]hÌ’’Aì€åGkò­|ráô«¶è*kàlœñ‰ŞÏ
<QtA6¿QY…ÁÁQ¸ğ°ÎÓ¬wUÃ_+olëœ1jº»æv'e]µ´„mÿÙtO=òØ—YjK¾êà+Õ[ĞL'ÓşQûQóë‹¶ÛË—zØ”h]ç‡j’,xMyî“uˆ“3–×bÉû±ÄXqCdŞ4û‚÷ ´Ö/ØAK¿tQ‡ıRïAÃÈ•ˆzf«9«³ÑvĞÙh‹É$ü	MêvÁùÎßÖ/)|…şj0¢“é?,àİiôÍÔÿBÎ!÷XÊö³(2%BEÃ/¤şàHı{ü-ãrìƒxœ[Á}Š{ƒ3«L¨§‹—eæ¥+$dççY)•–f¦(%Lv`õ™ìÍ( 6•§¶Lxœ­SÁnÛ0=G_!äPØCbØĞC®-:Ø.ëÒs›s´Z¢'QEÛ!ÿ>Q;K±[9ğ=>¾G‹ªTRÚ^
¡M‡d&fÈµÏXC+ç¦mØš²Öz¹UîYÕºlpitåpI`ºV”Ú8«Ú’»Á’®i´eFi[Ö›‹Ù£ju­İD;ÊáË–ü·ÜSbsäOYØ´P† ë¤UO1Q	ÕËÇ÷s‘AÏÈu¥`íÙ›/ğ#€'éÉ…ŠäO1[¯?^KşÅ’¶¼ÿîÑ®æiÆ}„÷§ğ¾);!¾[É¬‘oÎËå mí§xVÑ“dÃÅÆå=Eæyöé€‚³òâ,!â)È*%‰²Å:s§Ú Y%_0eßµú2ÄaÚî"õz<q!Ãtuÿ)\Òdæü­õ¯˜ŞvÆ}ñÛ³`ÁúÑàñåÜæÇŞN1¶u…ÆÄç GX_bpÆ–û!lx7õW©xu¯z¼ëOà Dn–Kp.ŞÍÑÓá–Š›Şu/ÎCŒôµ2¶u‘Ğ™)#×Ò~ó?µ·şšÄ¨ü	lCÛìr!ß½Íùœ·Ñé!¿ïĞzİç-)
^nÛáø|*¥Óû,Í­xœ340031QH,-ÉˆO­HN-(ÉÌÏÓKÏgp~Ñw'¹¯ñ^+‹„ —îÖ÷ñVö{lÛ:¼öxœíVQoÚ0~Æ¿Âã	$HŞ‘x(ˆµH£`ìeª*/9‚Ebg¶C»Mıï;;	-ôm‰rñwwßww6)V,*Yf–£— RÃ¥ „'©T†¶H”’ê×†6#n–ÙO/‰òï.™úÍBîG²›ğ@É®$™?]E~ …6LßyÈ¯O1:j’Fi#“‘5ésıæîrOî¡Yd97<ş »<»Ì:h’6!¾¿f
éû¾u=3Ìdz(C˜°”öiÂÒñÈ…ùk5ËJ(ÿÃ¬„vñ½4Ÿe&ÂµŸ¥1©—{*_tBG/)WP «Ğ§ğ+m6à±X³˜£9DH`æÓq;<d
;°3ÀsJ°ÔW¦õ³TuÆÀSX(ĞËorbW©wåªBw•:)®{†œó	!Í n`G¨Ş;`rÂµæ"z[®ZÒz‡S¨Ö;
=÷&@ëcZ,Ö™:ŸÎö˜Tå{zgÍ+†òÄ_ñÒÆ~™hfTáN.uÓÛ¦˜uóà‹•+ÚëÓı·£şhñ…]‘Ï»“)A-®jï…¹’ßXË!Zƒr[.|%Ä%Wü[…[æwËİ™pK®æën˜AcÙr(²-v>¯³Æ;&Â”Å·‰MŸúTğØz(Så@0{ô¸Ş¸¶[¿W’m,{±ÈFÊÎîİÃóŞºÖ^o¢£İ7]5ÖĞ&oë4à"ÄÖ´"U´qgYùê.;‡ŸöªØ£Ä¶M›«EÍ©ÕÍ99¬Â–]ÙÜ×â,ş<—RsGfsÀ+_xã,¾6ÀÁ8®Õ¥tCõ¾6ÁÁ&¨It)µÇ””Ô)şu¿}¿è[mşÿjÿ~«jqê„xœ{#×!ÆQ˜œ˜ª°‘“Ÿqòb6éÍÓX¯q {[ë§xœ340031QÈÊOÒKÏgX(ê>Ïk’¶xTA¥bóiö}œ›$ô!*Êó‹²S‹@Šú˜úõßÆ%,•vgğwèì BÖåß#xœ›Ê4‰‰§ 19;1=U!±´$c#?# KÈ¬xœ340031QH,-Éˆ/J-È×KÏgUÒş¢äÕÂÉ#ŞŸ8æ-·©£!D]rNfj^	\eUê‚OšWŠtÜ_‹6(§L¸8AÒ#ª²(?'¤D}±Öôà’ŸÛ+n}¸|}p:TIqr~XÍß$ßÅü¼+Ûk¾=vSòM>*¶ª¦´8µ¤¤8ç³ÉêåÌOşkøí
Œ(Õzrk2 S…Jòë€*xœÛÃ¸‡qÂ
‘ooWoS/õ×lWĞ¹ë²{÷{ïŒD ¸ÕO·xœuÁnÂ@DÏñWX9 ¨”ì½ç«Š?X6&±È®WWU@ü{5DHím3of²'ßŠ/6ì)ËÄ&:pÌ¢†[¨¼‡‘>$zNX÷lC9´A¢ëøÄÍàuö»^šÈA¥1ŠyôF“‘&?ºÜu@Õ¿ˆ³‰Œ“»°^iªa`s&Ôç0œLK0¼@µØğmQí×¯€+À±¤€Ÿô½ÚIé÷_.¶kâ^£dEnÖ—…ğwâõÖö­¸xZë€Exœ;ÎtšI½ 19;1=U!±´$#(µ ¿8³$¿¨’‹+3· ¿¨DAƒ‹3$5QËšD§æ•d&'–dæçMÌšÃ
–šÌËh C°xœUmoÚ0şœü
/ªP¨¨£nû„„¦Ò-ZG·„n[/qÁ"‰©íĞÔÿ¾óK-lêV‰’Üßİs÷°"ù’Ì)â¤Q‹”®¸dŠ‹­ï³jÅ…B¡ï9¯İ¨ ¢ÈO"i$ïKıN…àBÂ“9?áa5
æL-šŸ8çUT°%;[±%‹æü¬b¹àgŠV«’(1@5)#s>*@Tñ‚ø—­–ógæœÏK5+´G*Áê¹N5P¬¢ß÷ı"tQ„b!Æ%£µšruÁ›º@cß;4-Oé:¬Õ\¡;íú-ZR?’6 £¹ ªƒ<æ{Šë"Pnñ¥‰ÙÛ“ÉdF–´~–ik=šh2AJ{
Ê¿kê…b…NEG|…†Úc^U¼sµAnğØ~\nÉdàÒ A&h®®ÓÙv÷Qxº7Ø¢lj}ôËtÑxAó%bwmÁ¤„$Š-¢&•ô=M”yLWÊ!¬ïÁPZ¤áİfñe<!èâEzõÅ!KôãSœÆîí†iBOÎo}2Ò§Ä
C'Ô\P‰³ûr3ù€?Rö^%»›vm r 3¡š•º8ÚÒˆZ¿yƒÒw´0Ùmò½ÇğÕHß‡`ÊS¾–Ğ:Ğ®Ô‚
G»>oºû£OD.øQ(\)×\³5é=€UÂÚğÕE…6¦¿ŸÏ±ÁÕİú£¤–Dß[ÓuË,,<7¶VIk.VÓæ#÷—L³8Á×ìª#/ìx;œº›F0 ÃLoqC ßéûûËë8CáÉù ¼†Ïø¼İùÓxvN“éG¤qÿá
ƒp»â ÿŞá\ê>ÙE>ZcÓ^/™Ûd´^™í´Çncñ^‘–-<åàÁ×³±õ?êŸé¶;îOâ¿äk*Ân>u¤›á~d¸› ’î–xhF!3aWbÚ”e¸·â:Ú±ohŞ(jxe–Êû†Š-" ˜2'µñÀN5%Ø@«nJ4¹²+xt¿i,ü°›”–%¥wÏ™cË½îÒÙºnöqiö{=çÒBÖÛGí=ƒíÃí=†¦¼|Qœ1w¢NÿñO²|Áj÷‹ñaÛêÈß…ùÅ¼›^XÔğ0
²tBDàdõô˜ª.é|üMN3ZBóÂ^[´Aüylÿ»›Úşèÿ„ÛãLà†xœ{ Ø*ÈQ˜œ˜ª0‘Ót¢§=_biIFj^IfrbIf~ŞÄ^ß‰+•8õ‹R‹òóŠS'6š<™]l²SÀäçì²“˜,'ïbÊæuÎÏ+I­(ÑH.©ĞQ˜¬ÆìÂ	Ó¡·9‹ù9£,ÈX—üÜÄÌ<=çœL Õ\œœÎù¹¹ùyV
›XmQÍØ¼–Í’h…º0¦ÎÚÉªz\*†J\œ©EE
V“ûÙDÜSKPQÁaç™¦ Rak«P\˜£çZTä—”_^¬ ²èÂ’Ò¢<…¼Ì¸kj ¶øå—¸å—æ¥ UÖN¶a‘˜\ËÎÎ¯––Ó™ÊUË •lS»;xœeSÁnÛ0=[_ÁÃ`]äÒ9At)šØY±[‹-9”Ô4úï£ì¶iÓ›,ê=>¾G²ÚËÁHïÚ5Æ*gè$„êCÅµtr+-öĞÅüD†,ŸFPiz©4Är­ßæ•é‹ZíÕ¬•t’µ*3ëUEfæ°:é°PÚ!iÙ#¾¨G‚¢752}*Ä£¤Ğ·(`A´6®Œ[¯kÈó\D——s˜å+<&q(6v¡§™–J×¡t}º-¡3foÁ Âóí	øV2¡ó¤-(÷	Ä%RøˆöJ9Ş*İ É#lîÄÎë
àŠŞüL?p%ªËlºI!¹zçb^dÓ<)ü=(q§4‚k1°ÃÁ#ÀØ¡«Úñú<ƒˆ8¢‡ñÅ9Ä›ÅİâçoPuZöL\£­HNËõı¯jáÏÍb½àglå×ï±˜¬ÂÊ»©ïÔ3øc+©§h}ç€£4gfû+'¢ñƒhärºTD<^¨Ò³=®aª|sèÊë|”¾6ÇäuŒŒU¥ù†»&ßk~[f0VãDÓ¹<Ï•NòoX-‹¨xmáØ"…`‘GNğ%f¬9¦ëÂâÔ.Øó9°†œwmeÖÆaD´ê²ËÕÑóöË<<ù„%VvÏvÑËÊf`=‡(-¼şcP­ÙÄ²Ö£Ìâ•ˆÆı`:ñ,şXI=£éƒxœÛÍ¾ˆ•£ 19;1=Ua"§´xr~^IjE‰§RJbIbRbqª~qaÎD+g¹ÄÒ’ŒÔ¼’ÌäÄ’Ìü<ı”üÜÄÌ<ıÜü”Ô ò‰Ú¼ÙéúE©ÅùyÅ©JŠLÖbôM.©P€šªç¡u&—1jN¾À(3ù'£ìäP&G^¨ŒP5Pv:“:“&×d~f-N˜yz“-™M%¹8k¹¸€b%¥Ey
Eù9©:
y™9\µ\ AB„µxœUÛnÓ@}¶¿bX!ğ¶®#^Ay¡Mi´"ñ€ÚÚ“dÇëì®{ê¿3{q.V<ÙqfÏœ™sf¶åJ,Dg—Sl•‘Vé§4•ëViYš°R5-£×…´Ëî®(ÕzTÉ•<[
ı$*9Z¨³µ,µ:³¸nkaqÔ®#¦UAwÒšZMx÷ñU6ÃR¦£|@;+U‹`Å
Ğ¸éĞX¬Àøï¢©rsöµÊcŸrŠ±nØ%B…sÑÕÖaù>¸Q¶•îE-+¢¤ó®)!Ó-œèm'ø–WVÚGˆ½(ÎÃ3ßaî¡&YxÉµVšÃï4!ZSÏvŸl,ˆ!2Ö™&0È4c7	= İDõ"ànóY§Ésê\O„ÒX2rVÜ	ƒ}{©sA[xÜ‰vèÃúù¡ƒ¿yÜRı ö)|ŸÒş(c9¸§˜h}İxå<PúÜÛe¿V‡£%Ş£9ÒŞ¹VëƒÊ 3¤hñ ³/Ÿş.ü°›Cıy<ê{‰¶\æ6ÿL˜Mı¥CıoIÒÙäÓäükä}9½ıÜÃ|¿šL'äˆ_=ü^¾¡1ÒêÁx›¹ã¤ñ·ê]1ÛÔïéÉú|9Xİ!÷Òºó/ÆN"¯ ÕrEãVcĞ2,E¾«€ÚĞ`i¥"oC*óìŒ9•¢‰»â¼V3šòä^è¾?CëÒd®bÜ£,´„>ªgéªtÁ³R4Ù+Ãßùÿ­ C‡×üë}ÚwùA´-6U~çß:xæÖ£“5Æ‹º]Š;´²uMë^1+.bğt˜V”xVa-×r7‰}¹‘UÜ‘ÅG%›-ŒÇ)Ø›G(—X®ŒÛ%âp¬ÿÇìƒÁşïEw§”oyì©ÓSµµ´Ùp	°œ¹6l††?¿ıvó5;áG<j¹¾ŒÑIº^J\ªºBíÓ­é¶Èz3åPcß'Îƒµ¤·Œh[©œCöa~ÈŸ$7{ÉàâıT\[%2yúÆéõØÇp•‘,„À8‹N/U×XZª´=ÿ1ŸSõp0¢›0Ÿ±èuwt\£Yæ¢6¿tUìµ…œóh	§È¾›xœWmoÛ6ş,ıŠ›ĞuVáÈk»íC0èu:©_Ö)#Ñ6‰t)*n0ä¿ïHJ”%Ù›Ónòîx÷ğ¹ç˜-IîÉš‚ ¥ÚÌèVL	ùèû,ß
©`à{AJ¹#_³ §R
Yèo«\áã{!rÂ8k¦6å]”ˆ|”²{v¶!ò‘¤l´g9K¤8S4ßfDÑãŠJN²‘ñ¥&À()5§œh{¿î8¬…XgtT–,Õ;…’Œ¯MÆŠå4ğCß R7ÁGÆoHQì„L¯(_«¤tÅ8- gœåeÛj2³ï{}—1üæ›h±”õÖBˆùFƒE‘ïÚ"ÅÏ&ãµäe¡àQx(Áï?¦ $AĞ
È_Côèe‚‹aÈ² r*Ô{QòÔ%ÑZƒ½ÌhJwƒ@o
Vz3pq.ùÉXª·]‚u¸C{í¨•”:ze|B89§*Î·êQÇá$§.ş‘íöÖ
ª€j3s–¶ZXÔ	NMÀ"­¸8T'µCëÃäòVÔfµĞ”¡ô&FBBb¬÷Œ ÿlêÌ„¸/ Ü±øİ5µñÙN¾‹ÚP&#~Áv Iv0ÿtå¯JÀ@ná•tıˆ<pş¶ŸB¼ÚkıHÛC[gÿtn0$‘T' Ï‚¯%•˜€UÉÆ,×9&¨0gŒ”c
ì6uz¨;ŸŒãù‚y|OÀÒ¡³ºË‚½Õ›‰¤(é-Qh¸M«ïğ~vıÑ8ğùC<‹áêús<sµ…xS/^U/£I©lö&sß3Ùb¯±W<2«×Ûr!”j-iÍ¿fß.şŒL3±ÔÕ+‹h!®ÄÊ&‰0š'„kEx©×¢Ë‹!ØoKW²ııÆnŸaùõÄbğN9w.h™0EN64¹¶B^#x;¤Ş— 0	5‚€å¡™®p<¬B×TÌ´Ş¶‡Ä+%Î²aW||ïÉùş0Ö&=½…Ù| <ÍPöø#„]Z:i¿6×Iµ“ÿdºÆ©›ˆ<¼ºyÍ¼%Ë¯Sß3–}š=£æŠ Ğïp6Õıˆ¾e‚7‚¢tV8d#¬ÍÒ¢¡ØË^h™Ííö7íšŞõ./Î¡ú§§Ÿ‘›P
ÏqáôŒ¦HÅ0Z.&vÿIÿgÙ£#àLæ¦Èk9-³¬Ã˜Ö<?N`mVSó¼,´Å“%àåÊ4˜Ã–8ø40Ğ6¤Ø S†>Î9†B¸ŠCyPï„ğûynéĞ³;‹5˜WGú€çZVâUèôBíÔœgSèğºKlÛóZã>,ÎÁ~
aèprZ«¦ÑbD‹dxµé£&ªkí4n>ìièäz9]^…§Š¢§M	6´BÒªSÅ®u¼í&'c•Ğ½4AÃ4Â¢f6?àçcÚãfoC4.Ø‹>'X‡0+õ#× VYbm_üº.§óx¶À‹ë
ªAwÌ¸ÙÓWŒĞÅùûİÕ2ÃàÅë!¼xƒŸ·øù?¿66³x±œM/§Á30Ş_ì­üï¥Ø
«Û¨En^´¦Igô´ˆâsÇ³·Ø™U½éÓNÇFÙé482ğWÇª&‚ÿ…ëœŸŠFKN½ˆv2ÿŞˆ0Ó@×p²TUe©Z´8Qîù¸UV%WxÎ¾’úÏ³£À;ÌÿëQgan«9Îs{ó3ƒ®ıªkÚlysñnÛs‹óxÑDÔZÔzÒáÂgjõ‹³·UK~Ìu2îğÂ÷n-b[Ië=äÚÂ=ìÙºÛNºa|Ekûv«»æ8³ŸõÌ?ô8¹{ì“Ûmİtn˜|øÏ¸gQÊÅÿ>ş4é!ÒåÍ3˜b³¿uMu*G½•¾ŸÿÊä×Bä#Š0xœÛ§Ü¡ÈQ˜œ˜ª0‘SZ<9?¯$µ¢D‰‹S)%±$1)±8U¿¸0g¢£3_biIFj^IfrbIf~ŞÄNß‰1ò<ÙéúE©ÅùyÅ©Mà›,É",š\R¡ 5KÏBë(LÖa1˜¼”¥t²«/TT¨(“ ¨<ykæd}6N˜yz“ÃÙ2Á&r³â0Q†İiódvF$Mu@/9x‘D…7sr†1¢ÛÉUŒP´ù2—"š‚Í_¹¥°Ú[ZœZ¤0Ù]dr ¯.Šİ²hö®ãíE·×†ÿ&!SõÌ‘Ì©g’œ\'ğ#Èâ&óóŠO`S
H,..Ï/JÑÀnlifÊd^yå¨2[[%%…j.N %%¥Ey
»™EvåL^ÅÇ;ù»@1LÔ˜A# ÈQÀ<âGxœëPœªº¡CÀˆK__Á-µ$9#´8µÈ©Dzº(¥–e¦–¥+”d¤*”’*Át|æäÕœj‚Z&Ç2J1©jFmÈäeŒêR
nAş¾`mÅ
á®A®
™)
¶
*†J“Å˜Bä@êKS‹*uÀj<]4õ‚“ó4Ô@<½É2œæÒ™i
©EE
¶¶
Å…9z®EE~ùAùåÅ
Õ\œ“y™5˜8k'pŠ‰p\Z”6FG!/3‡«– !yGşªxœ31 …´ÌŠ’Ò¢Ôb†SÎÿU\n¶&mË)†¶l„
ug°’Ì¼’Ôô¢Ä’Ìü¼b†Ã®mºÑ>±C®ÖF.ë¢OÔEí8 =8§xœ340031QH,-ÉˆÏÌ+IM/J,ÉÌÏ‹OË¬()-JÕKÏg¨7aÈîŞzû´Qÿö-zÿ¬fWRs 2¬ ¯xœ340031QH.JM,IO,-Éˆ/I-.ÑKÏg¸¸i•ó„Ë¶{e\$ûjÎù¥ ’ŞW§xœ340031QH,-Éˆ/-NMN,NÕKÏgĞòø^ìºl›TÀ–ıÛU7ªÍbúäcQšœ‘˜—_X\\_”Ròæøo§€š¯ÂY7ÿ3
):«H×‡©ÎÉLÍ+A6ú·zÀÏjïÃ¶¸›#Uû3TqZ~Qz~	ŠÑ:³—ÔF\İ#¹vñ±‘±½îPÕE©é™Å%©E e,ÎÓ÷ï•
¶l<<_Úî€}3©™Á”åç¤ƒÔ<™´×v£yõÁ”ÀÃâ®7N)ÏxUSœœ_ŠìÈÒ·şìÏ>_Ï•g?ç.”â3…Gª¨ª¨YmÕ‰e‘G·†¿î±™Üw¾¼-;gÆ :”‡Íîxœ áÿ××Ç5¾œŒ-^èY‘\•âÃÒ¡yÍÎÁ‘Û|î5ì¼5xœ­‘KkÃ0„ÏÖ¯9”œøkÛS¡”„œJ	[yc?dV«&&ø¿W8ZyôPê›íİÙùfPd(8ÎWÁ¢ºj±‹ÈBÕ”ø¼ã%Ò—Vød*Ğµešs÷9S¦JR]èiÔBª“ÌL+­ÈLı"0&¸c¤Êä µöïk{KÒ^m$" Öªüƒº®ê=ÀYp"·Jw ’–É)–{6Æj6ÔÊà	Ìç)°)àLêÒİ—áÔun«“‘›yÎ–·Ç}0ei¶˜.L‰vh÷ıÃ#é:W+ùŠÛãÒ8€¼Çÿ—Çø2¡_r‰Åäâ÷éfß;ªåÃ±3ÿeÀ1Û¢!Gx8Y„Ç£{Èó+æ¸ ‚ù­öK× yÛK¿Gßê|1ßf*{è8xœ»Ãv‰£ 19;1=Ua"g=siIÆD£‰8ùÌÔ¼’ÌäÄ’Ìü¼‰çl@r“uÀ´ã0ı‚1GH»äç&fæéy'¦e'å§”&§épi*€™\Éô ×F&V³Fxœ­’AÓ0…Ïñ¯z@É*uî+z¡eE%¥Ü3M¬&va«Uÿ;N²Év%tÅ%‘ÆóÆß¼çNé£ªœ
\ï=®•G!LÛ9bHE²ĞÎ2ŞóB$CË†,*Ãuø&µk‹ÒÍ²VtR¥)*·l&·dl»F1&ŠÉª¦ÄEÉ.ºBß««¡ïœõ¸™‡`5¤AÃM·ËàÎQåø“òş§£2Õ|[ÊõøÏ!’ÂÍ´±|®øŒßzÎ ısÇH9ÊàA$æ Ú4–S[o`‚•êĞV\÷¢„Yxı—‹ÎùÓ¤wDÓùçvuŒR$g!’è Hp»‚ %aç¼aG'y‡¬ûüéí©ÿn7½-ƒr¿ßn²¿W¾Z5ÍµlQ2"ô#VS’_‘Ìá4§Ğ³ÍÆÈ“±ÕxÿìÖÌ°zÃ¥?[ûC5¦œÚÖÊZÇ;ÕâHÙ=Öß+_Ï†õĞ}aFş=ÜËŠÊKƒ‡³ïÊø¶/ìS29øÁ«ô’9ûoyı«&ß±âào)`Õã”³øÿ‹çç‚$xœ{Ìñˆƒ£ 19;1=Ua"§ÒD¾ÄÒ’ŒÔ¼’ÌäÄ’Ìü¼‰¹&;0Jp–¦—h*hhy@,7ù8c-˜NfJÓÏ™êÀts˜fÓî,2 8±!7ë	`xœ{Äq›cÂo6çŒÄ¼ôÔÉŒŒÚêf@bqqy~QJPjaijq‰¦‚†VbiI†KI¾Tm0cLÛnÆ©0fSÒdS&i„ŒÂäŸLü“…˜¸ıRËa¦NÖfR…))eî€1™YtaL3 Ãy4Á¸Fxœ­’Ánœ0@Ïø+¦œ bÍ½‡–$Ò^¢ªUz­ãeG46É®ªü{f“F¡©¢ö„ñŒßÌ<{Pú Z¤F¿¿u¦VÎı@ì!IªÉzsôiXfb7­ZôûñNjêË¸Ù+>©Ë–6=j¦7ıĞ)oJ§Ùª®œ”õ
mÙScº÷†C›Š\ˆ{ÅS_e	WÌu‡Æúò×4Ú¤”"y½]Aì\Ş˜‡,A°äa7…ÓüLÛÚ{Õa¾ÍÆ?!×b/¹KèÈwsN€‡w£Õ.Æh8‡OÁF„eÚa±,ëø-Êö²X@à<£msÈ.f“—³HEl$‡Ÿ"ÑÏ;ğ±‚QK69ôÄ'yváó©^*LõŸë¸›¨Àb7!“Pd;ı¯­‹äQÌ‡ÂÉï†qwú¢œ{ n²H•Ñ×y”üôŠæ‰N;ÒÅãª×šMx-ï1[ ›Ùhûuû4ÿŞDM}Oö¥äÕÒ5Bk.¦í?¸˜3®è¼{ÛÅÓØwDsáÿò€–'@U-wõ'd€ç‚'xœ{Áñ‡£ 19;1=Ua"§òÄz­‰	œ|‰¥%©y%™É‰%™ùyó&Öêsé¥äç§*MNdš,Íl5YŸEu²0“'LFo²!S4¯‡©o³4³/# kİ#+¶xœ…ÁjÃ0DÏÖWls(vHä{ 'çÒK))ı UŞ8"¶Ö]­hBÈ¿W²
¥P]´ÃÎÎ¼ÑØ£éÈD9¼lL@¥Ü0”ªXXò‚'Y¤±srˆÚÒP·îèÖÃgÓºº£õà,ÓZp{#X»tÃŞôõ[·BU)µŞB-,ãÜTÁ;’¹´r‚[—næé–SÆVH'<ŞágÄ IVPş^…‘|À 3qU¤4l ZÍ8RpB|ÖcBÍ‡ùL05êõø¼ç¼÷fÀY½š¾ˆÛJn?Å><w}®*%²‡Ç?¡.×U6ªâªş1¦¤{é2ÿCZİ	n«»ÔoÂÎwÙ‘é7ßäÈR¿Ì×7†«úô¬/ì%xœûÆ|ƒ™£ 19;1=Ua"g_biIFj^IfrbIf~ŞÄ{)ˆKI¾^hqjQPjaijq	«© ¡5qÿ”É!Œ“k'_bT E<ëVxœ»Áü‹yÃ,F©PO+(-N-ÒótÑ.)ÊÌK×ĞÔ™<QI"–ÊÏI…ËëpqÖê(äeæpÕr wÛº½Axœ­SÁNã0=Ç_1ä” (Ğ+	q@H«=°­ö€ĞÊ¤“Ö"ØÑØ¦Z!ş±–„-·ÍÉ“¿yoŞ¸—Í³Ü"Hïvk‹7Ò¢ê¥7ä YD†lÎ'ëHé-K!£mÌ_\À/ß#y‹u]‹ì3\BnAK×ÇªõPrŒø*	ÈtøÕvç8õ"û‡ÔòQi÷6^Àìò²Jˆß¬ïBp‡;{K(Òj'5:OÚ‚# ÚØ`ÊÂ6‹Uüw.Z¯(|ç>¡œ‚ñr•ª!1+¡x2¦« N©„·¨òì“Ór„V+ÜYÊ0y†År¤ù!6x=ã$ƒeI ´²³8t±õOÜù=ºhã 5^oòRd<‡Ó$,²k›	‹ù7,æÿ…ÅpmWplªU7øua’[™ eÙ±³g;ö‰~U‚3`t÷lj&ö”i_Ùuf›Ï»wp…ù¦d—0¢ë”øg£Â`•°TÃå@–ÙLœŞ®pÜ®ãV…¥
[Cğ§³ŠfIÍö+ù`^i½2?8G·„åDZ¬=Ø”pø<›8Íú? &Î\½xœuÍNÃ0ÇÏõSX9%¨Êî Æ„vÙñ QpGD—'EÓŞ¶)ŒMâ+?ÿ?äŞØw³'&§·—Hk	ÀúÀ	%TÂŸhHb‰9p  †'ºZá†yë¦s¯Ï6ô„Zk¨n?,Z½£O)„qbBM~möe¶x—K…O”f±´iÀ¥…^—·F¦L±l`Lìü^¡,C]Ò ríõjÓ ¨æt¼o0[ÍÔ‡èRà/=Æ>RkrwIWã6SÊìKå½ë :ÃlÿÏ¬Ş.¦8y\7V‹ìt±şË~ õí¥áßjé“ùè€txœÛË¼”™£ 19;1=Ua"§²yzfIFi’^r~®~Jfv¦nFbQebJ¦~z¾nnfrQ¾nIjnANbIª~Avº~QjqA~^qªÒÆfFiOÏµ¨È3¯,1'3%89¿ •«– ¨4%¶³—xœÕUMsÚ0=Û¿bãd@¾w†CÉLn$äÚò*¶ä‘ÖL§ÿ½’l°	é¡ÓÒ¾İ÷v÷©äbÃW¼¢õÜâ”[ŒcY”Úâ(Zn)‰#í¯ÌtÁ¥‚d%i]-˜ĞEšÉ¯¹ÙñL¦+=.¤0zLX”9'L¥‹7Šç©GERp’Z¥Y@JaîĞ¯@,7««Rƒ¶ÔÊbã8MÁq5÷[iÉ‚AªŒ²@¦BK¨Ü`8‹—•0¨ÜVµ8ÃNä@ĞØ´ş…pÅKFªÕZçğ+¾ /¨3Xj+I›{*ó w»yê[ aÕ%†ğÉ”ÌãßÅÔ #èÃÀòŸhƒÂ·šiÈ8ñ…ïh6´Ÿ†Ñ9>ÎÚ*FPrkß´ÉÄ·™`ÊŠÚ=ß¦èc²mÚ©.
­jªç“O˜¾l/ru¸â@ª(3È şûe^¶ÿ…ÏHßù‘¼A€ı½>’˜ó}$N¨VgIuuYyø‹L|G{¸¸º¿Èpe¹ğñ¹s-ü'ô¾Ö~j c{nfıå>ûë—-åúás•< ‰5¸"êäQ]ïgÈù£½	.ä3ìµr?j9|ŞéÅÆåå’·]µ¾ÛéÆ³½¨ì•ç2»ˆùŠF.wGxÈ9<«Ï“~ıäê´é]~/Èôäïv³V ö¶·IõQ©¢C(LÂş·ÃúÔzı¼ÌË¯c¬÷u:ºuåBœ<]‡…hÅpè^ $éòÙ¿­ìŞ˜)WJ“[•û¢¤Ã5’ömÕqÁ§‹ÕÎ^ŸÎ:>$ÄÕŞp^Š¿fñÎÖPœ«xœ31 …äü¼´ÌôÒ¢Ä’ü"†;‡
¾¿ñŞÄü¨«>ÍowÛ¿6ÎLÀÊRRs2ËR‹*8JdæÉxÕZsü”º/£ûæÍÍŒB¨’üÜÄÌ<Í›ÇJÜ‚6ç]zªş‚esÏÜ_>P%ùÛ7í	l‹ÛõEu·Î)Î<©²MÙ’Ôâ’b†0
Ç3só”¦N¼ÆŸ¼câİ¸˜B¨îÒâÔäÄâT†p	O³ë¶Ùëäß•~ŞùùVÿÑİÿO Û‚Të¨xœ340031QÈHMÌ)ÉˆOÎHMÎOÎÏKËL/-J,É/ÒKÏg,9ÿ{—|Á‘5*¹oÊÄ~Í=	 EåÍ»zxœµUMo›@={ÅŠCUÊ5R/q*;—*JÕ^Ñv=†•aC7Êï,`b“Kœ,»3ïÍÇ¾¡Rz«29¨‚òEz»pvc²9Â”•C’¡˜ÚY‚g
„˜eXéUëòûZ™sYqæ
e³Øa–øó¤Ãl×i·NŸ®½÷Vm¶judüUÃBÕÀ@†òæO¬]™¬ÍÖÌs…;µ6Iææ¥ÑèæeU(‚Äp(hUÑ“¤Úã$Mš’– bV¹š2„ú3‰÷o¹©¬î~&33¤kƒoˆÅlĞØ%7âã„k(Ìà®ík0"XU$ÈnLpçJeì(Zœ1ö¥;ÂèÆnP±œˆÉ §‘ÿ‘sE´æ©ŞÛ"‚vH=P¤¬	Mò…áµü:¦ˆïKñ*Ä¦±Zş€¿CE‡ïzEò¤ÚñĞÛÓ"PƒV~Fõbô4úõÀ2ËĞ ’{5=Ë~–xpÿ$ vğçdªåÍ79­ß˜sì—¡zÓèÌ¬i±&&Ğ(:§ÚÖyJÌcïñåjOoÛĞålâW§¡òŞ™À<a’H/Ên<ûÊ¢+
¾xcj?âîVôa>†ÿh$£ßCü™©Y(İÆO@w×&I÷/"X:n†Ñ•gÒ'é——Ÿ®aäeåSl!¿ëÜù1t„õ;÷¶&e5·…9]S…¶·Ü‡EyEşd½ß{EŒN£·îÂÇÈÑ¡Bá0%Ÿk/5k
ÖfpÈË®xœ31 …ô¢‚d÷œf7l›_,Ôçu|Í"Şn–Î())`à›vŸ7x¢cÉìü?ı¼¶‹'~ rûÙ«xœ340031QÈHMÌ)ÉˆOÎHMÎO/*HOÎÏ+)ÊÏÉI-ÒKÏgXiò±`§ôIƒärÿûª†3Š“;›Gµxœ­UMo£0=‡_áå°‚Š€zÔSZµ«JtÕ4í1rÌ¬ÌÚ&iµÊ_›„g“•–Æ¼y3ófÆ.1YãP˜©lšY?Š’8ÍK.òœ‘Kx¡àS¹^§œ§Â”3\¤!i”jxDxÒuFæã©¦z¿EçÀ¯z½hÖ‹Í­{\*¬*iÜ÷¢¼ç9¦…öAUV-CÂó(¡k:Î°øÂ	R>Î)|¬ /VQ…(0kı/ˆ¡‰’šÇu|ÇQ_% “¬àŒ@R‰Š(ôûÈï\ÂK@úD>pÎ¨äR¥dÿ±Øş´à:5^­ñ1Õÿó)®#PyyOÅ)Ãàm€kv³ª
‚bØN÷
yÇÊØ²RÿC’ÀªÉÕ‚vE®×Ã·`ÍÜ´1m#@U¢@ß=¥7-Í49©æœè í*MuÛà‡‚Ô¼Ãm¶¤oĞ–mŞíËïtsÈÆG5Æ#êµ‡D8mŞğ«©ĞMÿLè÷Ãkğ‘÷ˆ,y¡ë…@®ıiı6X´ZÍês-9gúl[ªHÖ¹g 6”€± f^]7@.fÌìKĞÚß!k6©ùaƒêXj5íC¡;Î¸+İŞZØSŠ­ 
/Œu¥Æ	v*KD	¬pÅ”1ìô’ş¦mGÿ]/æñsüò›6Û¨ ÌtPs7´1Nî.³Ì^ßÄÚ®Ğ·£M8=¶+Èâ—·=aÌ¿dŞO¼Y›™h2;7X7£wíHêvÕ‡†·¨©fõ¿™ƒŞÙÒÜáƒÙöê[8œúîfƒÄD7}Õÿv}òØùÈ¥¢xœ340031QÈHMÌ)ÉˆOÎHMÎÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgp>öÏ;©¢Ì5n×â'Ì÷î™~Ûö°!.­Eù¥%mv[³”LrJ]ûÄwBïí‡¶)+#a/ ¼'xœ•‘1oÂ0…çøW\ªD{é„Ô)­D[	¤¢Î•1Gb‘Ø‘s¦ Šÿ^›¤$H]ºùÎ÷¾{Ïn¤ÚË¡DYQ™—¨ö¢†1]7Ö¤,™$Q†æ„…¢ĞTúW¶•Ü´ôUiÅá!Ş8O¶–ÚÀX±Õ{=+¥;É­…ÕZ9;#¬›J
m‘•è(Ÿ*bÄöÂ™°Œ1:5Êr¶ªĞAKÎ+‚o–øsÙŞäèöóÅĞùè†Ø™±7
–ø•_aéÙ_Sá‰\4å¼3p?8Í_«sèS–œ¯–R5Ê—Á…ŸbNGˆÌ#”:g/;:ïØúŠ`şŠ÷XŞi³ø)ÁÙš$ù6NÄŠwåê%zwcHÙcáî¥ÿ¨5ººçè%†¸&†ùëzµLÂÆ[²ø!öŞÇ¢xœ340031QÈHMÌ)ÉˆOÎHMÎOÉÏMÌÌÓKÏgPQû ıùó‘òœàC‹ZlØ“ü ÆûÏ¸6xœ’ÛNÃ0†¯›§ˆzÕN[£I¼A'1‰Äñ²Ê2/šÖ%ñ¦Mˆw§‡1ÊT`p+¶?ÿ¿“ZªBjà9HKyšƒ*XJS1fÊñˆ¡ÂŠ`O!kbm(ß®…¥°rå©P9ŠİUÈíjµìXOsjDm!Ñhe¥tZ´yÑëâ¬³İ¼…Uòá¬µ)Ì,—î ×Fhœ•F9œ”µ•Â4]%í‘©–!Ö„!‹£C<ÅjcôÖIBÇ»úTÀ_Yğ‘HÑÍ&iÆœCÇŞ˜ëFu›rh-œƒÚ©c)wğ²O|2ÜP²ü4|×Ä<ú¡Ä×Xy˜ö’b<KRyt){Ê=¸]#z¤.ëP÷]şÜò’¨şÅ2o¿Àw+(yôJcŒ˜O¾¾ÿ˜õæú„½EOÚ¿¿B´§æ‡²^÷¯Ö¹)ä;ßæØ3§¯xœ340031QÈHMÌ)ÉˆOÎHMÎO)É×KÏgø¸ø`è»Ùf·¾^¼a™¡¤Á°¿C ¥ô†·xœeÁ
Â0†Ïö)ÂÃ«vV<‰°ZÂ:7š`ÒÃßİPV,šKÈÿÁ÷‡}˜ıˆÑ/Ã|TrNWFè¿á%M
¢Ï^nWÏ)0<„Ò¾Ëuƒ!€;Ñb«"6ğş—P˜’ 56î³zÍRÍl6)¼TY¥ \o¿6/I©ş ©BM	ªxœ31 …´ÌŠ’Ò¢Ôb†cvgn´±l·ÿº×’?:Ã$nâ—é&`%™y%©éE‰%™ùyÅÿÿ~°hûe“Ër²F.ZÜ8 O¤¬¯xœ340031QÈHMÌ)ÉˆOÎHMÎÏÌ+IM/J,ÉÌÏ‹OË¬()-JÕKÏgèî½?ë‡Ğ<±èšÒ•İM× ë'àæM€­	xœRÏkÔ@ÆİÕnÒ-+ôÒU»ré¦„„­ˆ¸Ø^Vt«¢èuÒédÌ4	“i‹àÒ£ˆ§0ÿ„Ş*úÿˆ/Ş¼ª'?”L2ˆ/ï}ß÷Ş›÷½ü\[]/p>t	ó§>ô‚´¿±BfèšÁğ4ôôÃ =ß {³ö|Eˆ@EÄ‘Qädu§ĞÉãyÏOÆéùÕôÇå~¥Ç=Qçß[7¬ãeIç |éËœ.úW3ÆâŒ0R|Q–	ü[ëö¦>N ç&b-pw&cwTX'GÎ«%A£„!
“šÂ®Rá¸.Ââ;˜Ö$,¥„€Î0­+¤|¿½Ô%B:?mï^0ù¯ö„ow†üAgSüWø«Îú¿-<BæâRûşßP×fòU¦Ãê±í’WôÿÜŞà_;a¶@æ{‡Çòı‹ïÏoóÌ“ *$p*ö³ÂÓ2aÏ~\Íæ
¥F¶"sVê˜Š.ØÍts
S—¬TıRÊõnÊ‹[ÍaEN1ÿ´ôîVÍî™|v'éHÈ”½şºût¹úı¥»ørSåaş{>ÑOrÄˆpÂ`	ú8Ô®µ_i$ñrS,UÄüÅò~©·Æ¯ôÖùõŞğnfRÏ=H,]SÛr”M~a}¡ÿe¼¬ xœ340031QÈHMÌ)ÉˆOÎHMÎ/I-.ÑKÏg_«Ù¬ãé¾ÿ€Ë‚ÄõÉMÓ¶  ŞC¶¢xœÕVMoã6=K¿‚Õ¡6… =Èa“ÍÆî"Î"v¶‡¢0‰–XË¤–%1
ÿ÷ÎPtbÙq²E½8äˆóæÍÇ#S‹l)
É„…LUr¬AV€2z&Cµª‡A”üø.¥ÎL®t‘şÙM-!-êİµûD!#ıE‡(Äu¡ lïyfVi%î@
©ÌJ“>üõ?7`%d¥MûbŠ¦‘Ş;Õ´
$*l¤¨ üvÂ¢Â˜¢’¼0•Ğ7¶Hé{Zºn=ïÖó‡âÙm.J™-?a»sµTÃRØµÈUZ˜áJeÖA®êJ€LÊjQyìyFi&ê¡~VOĞZùï‘)ï&]tpM&aëZ2²O©KÔfÀş
Wî¬aà]Ø‡CV|o¼9Ü„á¢Õ‹ûğ#aS	míÖqB¡<ú€IkÙé{%ÈD>¾'NÂ@-œçOgL«Š ƒ®ıüÒZc;|'. Gj>½m^glñ>í™ö“yÔ;Ì{(|{ 9½	ÓÀè%Ãÿ´4m•O¥Îo–ŸıXUwZAãb„•ß[ôq•ñJ¡zÜvæ˜lüZBiò+	¥¢VéÃ‰o}4 ª$Ï0cçÒrlBLbòû­†–‰ì×ãëËu]©ÌÕü×éÍ$qlšÚèFÒÉŒE jH¿,c½°‚ğ…ÒøÁ]ÒÒ>à9“e¬Qß:Ãºr÷a4›}·±Ì“'ÛŞ~oEµÛ[W‡)h››/äÑùò“Ë¤/Ó[ÿäŠihù÷EÌG¯&6Ï`bæ‹®9~§WÂ6%²{æpnò5?_c­Ó×Á“Şèî§‡Â”Ç\}ÚÉî¿ÿq<C6
¸Ÿ€¶§,ªM]ƒÎZŸ2Ö± ıfpà°‹¥øáÓV¸¯äVõ0WöÇÍÑ¼L:Eÿ¸â®ğQM>8ìÜ&“ò‚h<ü{ÆÏñı)¬iuÌÓ³BŞ}KúcäÎP|,‚¨*Ê{ó"«›L_A{ä/*…Rån#É½òì+eb`¢zÃ°–WÓñ4:×ùôòöÛxrµ#4Tr7‡qòO.À~;îôRãŠíğ+_¬ÿª#­º°Ã¦3ı_Ûs7ù2¹ùmò^{¨·­Ò kªÏ\¹ı?şŠ|iáö†GÏ_œ­uş›‚x­¯xœ340031QÈHMÌ)ÉˆOÎHMÎ/-NMN,NÕKÏg8Åß:átıñÏq§¾wÚmçnŞxÏÄ ²Ó²ã‘õ1h‰1»½Rã¼¡{Ù?õç²f¼·¿eBä—¤¥£ª‘`(`UÈüK»âÌ±—œK,nCÔ—äÄ§d¡*¯{ÇX_tó¯—Èé÷æ­M¯:ÛõL+ºjxœ­UMoœ0=ã_1åP±w¤œ’C¥JUÕvÏ‘×° Û2ãDÑjÿ{m`Ó%8ÕFZN0~óxo>@SÖÑ†CËiíCËY·ù9!bĞÊ $$º8}TâF`k9SCQ‰Nd-5¯´E£²A0£2äƒî)òBHäFÒ¾˜Y˜§)ª‰'^s£º1ª˜ìÁWÍÁÎ^`DcÂ‘DZØ>~»0Ì`c0ÿÀ-…‰:ZwtMà®-Ç÷÷¸3úQ˜÷[‚?ÜÂp"¤¶’Áş²„’[XKƒŞ®6–†]ï+%» z‹ó4­‘ğui±‹„PB0ì´¼–¾‘Û°œyt ìĞ§·%–Áİ¢q"ÙÁİzê/-şâ£Vrä~«õp'¶G(ïáªl_‘ßHÑ%¸ñ÷İ‰öR {)úI¥šÆÇşOêÇ%»„ø\ÏxbÔ¾
–åÁ*ç‹áée[/ù¤îjÍe•| HaVé¶zîÙ§%OYk½Û.ßJì›D¯wO~1é¡ç™ËÏ*aÖÚCw+ñÿôzõµ2ğ”Â³×n¨\ÿ(VÌ^»¨áËs¾×ÓC@Ã<NDM{ÿŒ¢ƒá´s7§yÏ;½Iuëô‡œNİ©xœ340031QÈNLËNŒÏHMÌ)ÉˆOÎHMÎ/-NMN,NÕKÏg(f>i±3ËÙûcÇeï#r¿3?˜‹¶"xœm‘MKÃ0ÇÏÉ§xìAZYÓ»°Ëºƒ A=,{š>4MJš2æğ»›¾Ì)z!ÿßÿ%TÔ¬ù„Ò„º¬Q5ï=–²GÎ©íœr–h
õpÊµEºEÈ“0×.áœÕ7ıÖµ’,üÔ©¡¼–ş,Th—·¤¼Ë¶‘²½•¦˜){5bŠãÄI~»ÿ"}çL_(g+Ò	Ï8ça˜@ü Âå“ój°
vxZª¥üÉ+ÿŸ.œyƒ·p¿€#ñÊLËu“.Â1Ö¨;x× ïáq=,ŞÊ—tN+6QQÎÇÉY”†â¬›Y"„ˆmX|kW€Şß[’&]Àb‡áä|´\Áõê5x²:Í2Î¨š¤wk°4å¹©¤‰ÿËb	¶‡5Œ.ÑŞÃğï¶q<Œ=¿ <æº¸¬xœ340031Q(È/.I/J-ÏHMÌ)ÉˆOÎHMÎ/-NMN,NÕKÏgdğó™~ıñîÇ;Wt?vçîÔ¶% ¥¤B°xœ}‘MOÃ0†Ïõ¯0= v¢Í}h—mNhâŒ²Ôk£¦IÉ‡` ıwÒ®-‡	¤,ûÍëÇvÏEËkÂŞ8_[rOÄ•ov‰öÕÑ;]o¬Ç’æ·º7—ÓZú&Ka:VÉV·g^IV›¢“ÂšÂS×+î‰IíÉj®ØÕåM6¬}RHştúòÆ(ÇfÂr î	Ã·Axü†dÖàjÊÃÀà´Àgú˜&Ëş‘çx3ëR»İÑĞÛ’VãıD3ÎzYï$—…$W“<ÇÑ/Ëñ§üä	ÉZ\o0ˆrá{yWŸûmßAê:ËGÑİµÍ'®âéb³9wD±óEà­«xœ340031Q(É-ˆOÉ,ŠÏHMÌ)ÉˆOÎHMÎ/-NMN,NÕKÏg˜şFù[å"‡w+’?<ãÔú,é!ı ş\´&xœu‘MOÃ0†ÏÍ¯0= u	g¤Ğ†Ä‡@§¦mÔ4©òÿwmš”ƒíW~òÚî¹hy-!tıZ¹[ÉuhVí«—+î%!ªë­‘$íyhX¥´‚.š :™Œk«¹©©u5Û2¿ó,µ”æº¶WÒZ…&¾Sa;VªV-îv¼T¬¶‹N	gAv½æA2e‚t†k6R6bÀ°òÀœ"}kµgÂšJÕ)É		»^B§\aÿMH€Gù9Í›åğÏ/}9±Ø“ÄÉó‰ŒÈšES9‡CÒßÑ×Ğ§*8İÑ;ÿìlyc>PFeF¢EI¤%›×Z:tQ f¶…«%LÛ§«ƒ–]æ#Å?”Šk?aÆóé½U&;f¨fÇòRJÙüRLâ&gôp`z-„ô>ÙÅX{Û<=ä°\‚Q×ñ’±Ê¢xœ31 …äü¼´ÌôÒ¢Ä’ü"·_m²ëEâ§¼¸½oS°üé[›æ2+KIÍÉ,K-ªdhv™´gâ|ùegnÆ=å^h4½%oçE¨’üÜÄÌ<I{¹ß©+-ùşí¼ÑiwŒ=ÅÃª $ŸaÑÅ¿Ï³Zkv|èpøÿçÿÕåçCdS+’SJ2óó
Öğ~Í2zécäÛû¤ÿX©‘'ïkˆš¬ü$†¹æe37¿¾txÃƒÇeÁ7Œİóô!²E©ùÅ™@T2,RóR)şî°\77™;`ÎŒ	ë9¸!ŠJR‹KŠ¾‹<]{°Úº¨h•qåµ‰¡ÇûAäK‹S“‹S%/ëxb6AwææÆî\½'*}Š ÉÈ¥¡xœ340031QÈO,-ÉˆOÎÏKËL/-J,É/ÒKÏg°[-·©ÒÍó]Ã£/…´ƒw°n>ë  ¶”©æ•xœ›$xA€³ 19;1=U!"ÇÆü‰3ÜÅóKK2ôSRs2ËR‹*õÓ‹
’•¸8ó'şÃÊ()) IMNatÓF“ËNLËNÔ/(ÊO)MN-©É»äç&fæM`4€ª€Ñbr–€¥äg–äUÂôM´7|€‰S"_ZœšœXœ
“4š'5™‡YƒÂİìÄÆÈ˜¿y7š¼˜ÕH>bµ ’Rl‰@²šM[¢T/(5=³¸$µÈbĞa¶Ã@ùìb“ç°‹‚YdÆ®xœ31 …ô¢‚d†È¤ Ÿßz¼yºğaS•”ûª6M–&`éŒ’’†ÈwÇnV¾ı·lçÖ3‡şç…nÖUğ†Hg'¦e'2|ú´ØÂùô®„¾»c79ˆ®Ñ~¢¼ ‚|*C¤xœ340031QÈO,-ÉˆO/*HOÎÏ+)ÊÏÉI-ÒKÏgx&½¥û˜ò?	åÖgï/´t4œı ıM¤xœ340031QÈO,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏghXrÉ»#ÔvâD¶ÌëÅÙ'ú:œS1ôå—–@ÔßÜ¸p·FùÂ¹í—M÷9²8r3 {A(ªâ
€ú(xœkeıÅÌY˜œ˜ª?‘£5?±´$câ”õ`†‚•­Bª{Q~i†’>XHI“‹³š‹“ÌÑğÊ”äg§æ+é(é%çç•åçä¤é…€E5Ñg‚¤&— kğ„Ëh²sqÖrqÕr “85öé}xœûÅ|•uÃWÆzÎT½ ÿà%ı¢ÔôÌâ’Ô"%…"½äü¼’¢üœœÔ"½ ¨¸&'BqrFb^zªnAbqqy~Q
ºg°t TEgZ~Qz~	Lg"†V7°<V­¥)‰%©º¥Å©Ey‰¹©è:CÁÒ¡PY ÎZ.®Z. ÖçOÃ¦xœ31 …äü¼âÒÜÔ"ïèâ;ïdÜÏ½ºìÉõµ5¡ªWÖ˜€•å§”&•¬éÎ¸-–&s]Ô±›_ÔarJø®÷ 	šq¬xœ340031QHÎÏ+.ÍM-ÒKÏgøĞé<9x×÷Ïd%6©ÖºuB”•çeC1±åÄëdÄœ)œ!`)Î¦ÿ\á í ë€¬exœûÍı™›³ 19;1=U!"G&k~biIÆÆúV “
Bë€¬Oxœ{Ïşœ³ 19;1=U!"G9k~biIÆÆŞ$f Y
"§xœ340031Q((ÊO)MN-ÒKÏg˜átwgÑ¢%šİ,J‰×Oö~P6 ®4ë€¬:xœÛÂ²†…³ 19;1=U!"G7k~biIÆÆ…“…?	ı«xœ31 …Üü”Ô†%üuŒºâÅ®ËûÄ¬/\¡ĞûªÛĞÀÀÌÄD!?±´$#>%?713O/=ŸápÓSûÖ¸z½ï*ŞuV.Kby
 ìÂï¦xœ340031QHLNN-./ÉÏNÍÓKÏgpqydıÇô‹©çŒ¢9ó;s
~?hUZZ’‘_”Y•X’™ŸŸœŸ’
ÒPqD]Ñº‰næœ6âe[ÚC5$çd¦æ•€9¼¶«ù|ÇoC£Z¶Û”S«¾öì€)ÊÏÍÍ[ıkw'£°ØÙœ;'?ìQøk”ô¾ª¨(5­(µ8áÌ[*'=¸D×?®vØ§óDeñ9‹%0µù9`‡Yôué™ÆÍ“14ÚÄßvb¹©?TIqr~XÍå¯»KV›œšW¹QïCì­«’†ÂÔ ƒ%â0¶ŞÙqÙ??Ï·_ñ_’Y  ñTÜA}kUµ—ELVÖsjà-¨šÒâÔ"ıŞÏEé¥'õ_±ìÒÑĞeg6 ´İ—†îHxœkcncğPDhfàYËÚÏçû7Du)Uj
/˜øu" º¯À´xœm±Â0†çú)¬LÀ {·
n¸…n¦!µ ¢Iz±+q:İ»ŸÓ0â!qôşOÖ=ì0ÙYî‡¬ >L)® 1ƒ{µL;ş¾Å2°Ÿ‰°s˜ÏéAYòì¡Ù§RÔ{ôåó€¨ÃÛã<'É>Ş°®­qKzñƒé¡ùbÊ|Îš¾Àê,à¦¶u—Ú”–¡©ZlÕ¢Õ«¸5R’âùxN>w‚e£íYB5¹X)ÜÉ%İóŠK¢Èü–`gx¶xœ};O1„ëÛ_a¹‚¡§‹Š4	©ï{VœØk‰‡øï¬ñ!QDLak´ŸgGNÆ¾˜ªh*?ï¢7 È§˜YİÀ as6ïÊë¬Å3yÔpÀï	ÕFÅL†)†mt¨
çjY}Â°ŞÇ ÷Lx¿S"	Y?Öy>r¦pQ“;ßkû3Éé	†SÁÜÑëp•ù‚öÜ®ºéïÕ¢U3¥‘^‹JÏúÕ$;¤ -ñ€2Z>öW—çe>ÖLxKâË¦•hÿ²~’ão0v`4Üğ£	ÿëQ ä|i†}Ïá‚3xœ;Ât”i‚{PjZQjqÆBF 9é€¦Pxœ{Ë¸ˆq‚ÿÄ0•‰Ê<!™¹©Å%‰¹\µ\ |	ºxœU1Ã Eg|
”ct-Kæ !@R·£ØUÕ»Š¢ªlı÷ÿ—³u»M¶ÈÍP´˜ ä™ƒ¾8ª›e/NôÔ™b¤ªó>ÕÅ´êÉÏ§ô)be»³ ¥¿´ÿñ£ÓP«ŒlÂbË&õÁL´Õó­ _}7jêş±>(±xœmA‹Â@…ÏÍ¯sÒKŞ=,êAÖ=ËĞÍÖA;)™T‘eÿûN;ËÒ\ß{yIZ[İlMÈ¶Óë–ë<€kZEC",Á <¬à²¢ÀRY¢¿¤û£mó<‡l†¯ÑÔ|‚W—@ò ‰mĞÍõá8p WÊùÇæ2ºØSÀNoÜÇZª¥¯”2'¬1½“é¹0ãô¬’Ã,a	 ¯–¦—EUºJñ²Íİ‘×ıûŠØù:ìûw§ğ£ªâô™oäßğDßBášèá›É‚oi…Qxœ;Â´i‚İBF o;£x340031QHLNN-./ÉÏNÍ‹O)É×KÏg?jğãæıéÓú—îv¾«[*%V÷Öª¼´$#¿(³*±$3?/>9?%5>½(1¯¦USWÜ'5üpƒ„GKV¡P“ùY¨ÖäœÌT Âä¢Ô ™˜SŒªõ{ò‚TßÂ[Iûf×8¯ä×ßnãÕš™WR”_\š·§NM¦¥…M=â„HÚäçCTäf=R*Î*/xä•IÜ®½S'¼­;”hê43„ÏÖª.?è˜»åï$|Ú¼íÊîŠ² ™l…õşJpAÕ$—ç¥ º¶*úì²"ûğc¿ÖÅDY.ıS"¤óê?TCQjZQjqj˜Z_z}êÇÄM¬÷„ó‹ŸÿÑç 6ä–Éå3xœkfŞÁ2¡H-#1/=5¾ ±¸¸<¿(E/=Ÿ!/v{N´×¬'ŸBB¬ßÔÙÿ7ÔgöÄ,'õ´ü¢ôü¥™Enï÷˜(ı£X²ätç4Ö——Ş‹440031Q(JMÏ,.I-ŠO)É)gKù¹s÷Ê:Û®]¿ßÙn;yÁ{ç+P¥¥)‰%©ñ¥Å©Ey‰¹© Õ^tnî½rÌãF…‹ÜO,‡§Û şêPyè€2xœh —ÿ¸¸VÒ§Øe?ÿÒ¨l£\İıøêÔ ëŠ‘jUîªy¡m¸ƒ™Â–¿ÒÅdÑ‘ÓÈ—£J×‚gµ9Ò1J9K„lgİ“¯Ï kUNÛ­UÊÔâŞ€²5“ßY$“3ã¹ex•TÁ›0=ÃW¸*Xs”C”ôC³Ò&Ûk–…	q62FmTåßëbv£Jå‚Ä¿™yïá6/ÎyLå½9­
CÑ´J‡AtlL}W%Ô,ª„9õï¼PMVŠ³˜r}ÉK‘UjÖˆB«™¦­s™´ÌëŒg¥jr!³i,çµçêÃJ©ª†¬ïE‰ˆDa†YÆ–E]·Wg/ĞµJvÀ8ç¡¹´ğìŒîÃş„Ákz³fî±Ÿ…¬ØÛÏNÉyÔ[è ÊT5÷3—è-¼VŒMësšâ`p,ıö»ºDr+¾jpĞAP!¾ÇYí3å$²îŒ»B¹¢Ï…BXóGÛôDœÈ´ƒÜ„“µ®$ã~yëM”<ö²`ñØíí>İcÃ=®”½?Wùó¦¬G@gQ¯”™ĞÒÆ	“°øÉ#¾™2ĞZé-µë»ÌìëƒZ[âÛ8gŞœú¥¶btonÅ¾…À8ş„’Càãó`+®a Ä%ÿÈkQâøãü|ÈæâQù¢zã›¨ûeÁ¤¨§T¾Êl1qÃ­LLL¯¥E€)ò„67÷Ÿñºğ‹Z€¶ïúŠ>¥ÿ	€ÛtÖ§ñ¯ ‹)·CN“I½×wñÄdhìã ¡~+Õ4öÿu—İ`Ü}B0Ø¬oö0¼J¸uœ uÁJƒ½ÂÊ¥™3Œ ß*‹ğ×ıÊáW,rÛ‡½¡¸sáYoûºí½Éw­½AÌ ß¬¢¦È¸¶÷ÃaG>¤íQo¾,Ë˜Z÷:7BÉxÔĞ*æfİA¡dIÍ¼’¸cèÈ/¢÷¨ık+$ ü°x,6*NÁ"pâ)„xœ…RÁJÃ@=h”Azó°V„Tëöä¥ĞCm{¨bÓâQB2‰«iv7V)x”€àÁ£Ÿã_ş€;»UŠÙ—™÷Ş¾ÉÇÊÃúÓÖ3İ]
²4bñ3yYMü4Ş»œÈ¾×ûï¯_Sw'óyqœ…Ğv€Ãì
Ò:Î3^#w-èwI³EŠ‚…t ·æØAâ³±@TOtôQµÛ§3!Ch°&Q‚ô/Šö´ßm’Ù3“¡ä,İZúB¶¥êB%=(ÆÀYĞõ%¸’2e‡†·ffz79ã pè¿™òÍj”«r%õr%,İ aJÚïj6/Èr0–ˆéûo¯ú:³sm¾ª>Ş;å§U™špèH WÉµÈ¡0dÇŞY¸:mL2†¸ryx6Ô"®á©³Ezàè˜²çÔ”'¸AªF<8HEÌÎ"Í¸Ù")Kp‡¶úPğum¦‡ëÜjä³Bş¯61W2ûh’íIUÛÓ~Ë÷ÅÑòlQåšuiéº|´BukoÃù‘ñçÿ$¥éÜ;ßH?Øy´wx¥UMoÛ0=Û¿BÈ¡p†D¾ì”[‘¬Ek¤ÙµUeÕÑj[,um‡ş÷‘´ãøchZÌ‡Dò‘||dJ!Eª˜ŞíWÎ„¡ÎKc‹Â`ò»Ié»ITÆ&©v{Ï¥ÉãD?êù^Ø‘è85ó\KkæNåe&œŠuá”-DSxœ˜\è"Î0?T>¦ğ$2gl¯Hk^_MŒ?óÆE›b 5&ÍTì½N–LÜWˆ•Ü›øé+šÎÕ$œ†¡{);ZŒÕ¯Âîª¿´¢põË«Ê_¬rÖKÇş„Yn0ˆá«.Rv÷³2Åb’bÌ-âMîÂ QÀ…¾¾ŸzlT¢­’ngõ É6–[o5aeZn½B°Yn¡å»ğ-|!Y”²/§š²Kå.´Ê’*’î™!3|i`šÏnz:‰°Êy[°³S©ÀõHÚ:€|üÂØü‡È¼Šº´Mgà‹Ä¡}_¢¼:ä-†ˆ=újÌ†@Âb¶¢ëÛ'i¼1´3‘<ŒõÇw»õ
h<î_’½ËÛÈˆD-Mƒ–êE%d˜
>¡1X¯ ×&RõÖ/X
”êfÌãš©Ñè >š2e-làqÄíVòCà–Ö!¢„XÈÑƒ4W%ÎÈÊÛ}A¿#.È?ÁgšÑ;(‰ÿïhåÊ´ş•ú}-à*Œ(j&ıáÎ˜¯”ívğPÍ˜z.¡Õj]0¸—3vĞçn³±J¸%õV÷%3JİÉYG=#¿	©Ù0Ò+tÑ‚¥U ŒäÜ-H~eÀÂw7ËÚŠj-£áƒÃÍAp¯í•Ï²şNø¶„×PÅaÙh¢ŞCÛ¨Ş=8VÛ$­ËúVŒU³Qİü<I"jfå-]ø¨0^ûo•4EBe¶rÙ¬ãö:Ã#ï-Îïp¯h˜@^‘¿	5yÂã
„FxœûÂ7YpC³ƒ{j‰[fjNJqXbNiªFr~JªBqIQf^ºBQjJfQjrIhQ&\,9'35¯ÄÓ* 9ù=£Òä &[_%]%.NNg V
P 2$„0È
ÙT°¨`M0Ó7«3û²  Æ(4jà€¸.xœ»Á9•ƒ³ 19;1=U!"Gk~biIÆÆŒëŒ›1İc ¿»sí.xœ›Êq›sÂ6çŒÄ¼ôÔ‰Ï”'³1*) AqIQf^ºBBVq~•RAbqqy~QŠR§_jùdyFñú¼Ôòx$ñZ.®´Ò¼dt-ˆiPÉ ÔÂÒÔâM÷Ô·ÌÔœ”bTyä’
…ÔäŒ|=çü¼’Ô
 Jì&(Tsq¥–”å)¨9+`STÀêébôÁäsŒÊ@hº[~QnXbNiªÂ+š:@Å Ï¼a”TEñH¦‹‡&«1-†ÔäL’0æ{¦»“õ˜e˜Afu2ûÂ„ï0[ êOw“·?x•S=oÛ0Å_Ah¤ —N^m$ğĞ¥Iº6u•™<—:I
ÿ÷ŞÑil'@İj€»wïŞÓÆº;€F;ÑzI¨”Ì¤U•Øì!èzğ´î:‡ÑôşÁÏÖ6?ÛŞ›gÑ»Œ3‚¸	–ÀøD“¦”›£õÉDiS«êÑß[Â|Ô’»àËyÍ^!ãˆC 3M¾—	ön$fbÀ­Ñ<~®U«=o@/‚‡D‹=¼ãe¶‰¾ÂÏ	Fb¾z¤<9Ò¿TU2×RÄ1Ÿ}{?bš×ƒT|—nõ­ª®2BcÔ(ql•ú1%§›AŸŸŞêK ¡GOZ¶ïÈ|¢öt¹,¦œôÙ©QİœkÖ]`ßl˜ 9¤Ø~bd!9/,ß!w4´ıOª×X¬Ô¸¢Èj©EÇîæfµdª{«u;&‡Ü>$…ÌcduvÆ-ùr’dµZ2Ç×ABh{¼°³"ı?èÃ÷»Â‡ór}ÓjÈ™¼—áÍİİŸÂ«b¯¦”Eöˆ¢;ÿeUuV²İ›ÿw ³z6°„‹:éQ„;Qß²p¿’šZàí‚7xœûÎ¾‹sCÓäzFNÑ°ÄœÒTâäü‚T…â’¢Ì¼tM­‰'ïe´’VÒUÒáâäÉZ)€ X!P¬vr £şæ,¦^F äMÓ·PxTÁrÚ0=Û_¡á±;ÁºôÄ-IË¡—z%BVŠ­uå5éğïİvÁ„Nâƒ=³ûöIo÷­k¥·ª0T‹›BÛª"‰£û¹)Å¨°¸i×™†JævkÇå*·²€qeµ‡1šª.iïT)C¹Ì¡RÖÉŠiFq´S¥Í‚P¼¾‚ä×¸ƒXp„¿<¸ (J#ÛÖæW™R­$%ÒèÈİçQœÆ1j#æ=4µÑøİünMƒ¤Q4è[âO-`kœèŠ[Wˆ—_¸É97zé@bûJÚ¸ø´â£VÊüÇ?[§ERˆO·NOÅƒOÖ”y“hÜ¾r6êÚÓÛ%|So°õNÜİ¢¤ôIÉ¤W"ˆ9{_ıPek’NKzß{5“›À=\rü€¦£$º´Æá|&xJÙr9Ÿ‘¦³‘²iÈ_
z“d5S¨*šÁÉ–™ZÄ!NFóİ»;ˆ%‡—ÔŠ‡üŸæSCØ€æÜG'©0Ş“)ÏşgÔ¬¯x®I;ŸzF„iÒÂDÑ]ÈfÁVŒ¹ ±ÿ¬79‡Ã Ş«ïgôOÊÓ‘ràô¦×‘eÙÛ-èrçxĞhw†³(éÓù_…8/À³Z%~†æo8~•ååÇCOó¥ù_AOãZÙ|_6ü¯¨ˆ|ÈÜvñ!:´•Ûr…›L;Ä?îkêxó€‚V“¯ß)3ûz <Æ$¶Éåƒ#xœkçjâŞÀÊZËéY\\ššâX¢ ™$²Šóó¬”2Ktòs3KRsJ*•¸8ƒK“  ¸¤(3/¦°¸4	U¡Wˆ'V…Y%™(
k¹ úĞ*u¸xœ+HLÎNLOUÈO,-Ép)Éçâ*©,HUÉÏNÍJ-,M-.Š*—•&—(Tsqº%æ•„€Ô Å2óÒ²Šóó¬”ÒAâñ ÍJ	\µ\  5ê¶Mx¥“MoÛ0†ÏÖ¯|(ìa±.;åš E†µÙuUeÕÑ*‰,um‡ü÷‘jœÙÔÈG/ÅWd/Õƒì4™âf1ãz‘W¬È±ÏĞjËËÎÄMºk8Ñš3ÛÈğ,[#:˜9£Ì¢v½•Qã£^Z‘‹œ4^8’)Yñ(­ie„p"‰*ğò‚~³bÀ#\¸è¬)™v’±ònˆØ‰ĞjâñSÉjÆâs¯ù9¿ ´WAúøUÿLzˆØ&bH*òß¬È™b1f|ÇoàçeG'¾“HyËŠõ@M9Í'TÚÅ‰kM™~wb®`%úN«G`ËØ}òŠWÿ0êMî^ó+/¶íP©øÄ©çfhûS¬Ï¢Vƒ)x~qF‰ƒsÚÍ%÷MÚ¤«c?êHÌù”Ü{’¹±Ø[nïKæ²3á7©üêAÛÿóçòÔVÊíãjÉidšõzµDSİ,rşØ™7IòdÎáD¼îHVF»)DÉbµD«v…È—íé=•¤q;ÿ–è.-„mBºª¹WäğdûµiFş:p•å©ìÈ£ë[9Ûì'œ¸#ÖÁİR8?Á_4Æ×~‡ÄØá;$òŒüã|3òù*ıä	‚fxœ»Æy{ÃU&[÷Ô·ÌÔœ”â°ÄœÒTÒâÔ¢¼ÄÜT…â’¢Ì¼t…‚Äââòü¢¸@qr~LzòyFW%]%.NÎP¨N+˜! á ¨~ 0Ì(p0È €Ü¼šI›	 »j2Õ±˜x¥UßoÓ0~Nş
S‰)A­ó/•ö0Z@EbH[`’kj–ÄÁq¶1´ÿ»sÖº]LTÚºù~}wßçs«òKU‚0ªwë¹3q¬ëÖX'’8­j7Â¯R»uÿMæ¦Î
}©'keªBg¥™Ô:·fâ n+å ËM³Ò%Æpº¦€J<!\7l£ªŒÃ³ÂÔJ7YMi†£½$WªÒ…rÆî`@ÔæöÖdôk2¸hÓì(M¥šròıÚeôsõêİ”d}¯‹=K¥¾u§šA¾6ÙÕK2;]Ã(NãØılAœÁÊB·^šKhÎàGÃÉ‹ÎÙ>wâW½³ªqKrÅë¦_¿w¦™J2}¡4£¯qfÚs´¾ÈGUÈ÷<7>áÃ¤™Ğç.W}“‹¤/ÂÔ[©xî­†ªè’ÜİêQÎòvãÒÇ‚¨%®·8:œ¶MO	 &—o­?©ª‡$l;£s˜fºï¼Û:ûsóœø@rß>ùİıÓ–†•ä•†Æ-æ‚T /.sÁVùrÆö°ûFj|fê©õW3ãDéˆŒÑbı…¨ó»]˜¹"é<Ê® „óÂ€$`-^Œ-3›Ë"ïcÎY	W Ê[ .ˆ(:b«Ü–üG¶PĞ1óğ‡!ÄÿHÃLÿ%>İĞ|
×ió…Å^P9}vÇpİXÀM‹}v‹FàVÔp!S‘„¢+a±|OQKÓc/'D–¤q”WJ×òº˜ñ¿$Œ3(u‡ë
6¸§äşéFB${üU$r‹K%aN¢E×õPœ8¼”+Ÿö5XÏQ9	í.yj¼XÎ’ÔÇ¼ñSĞ?ÄÈ“¢ğ‰æ½U×m²™ŞÁ5ÎŸÂç'•GşşĞøÉó1»‰—|‚‹ÍC§3­Ï©Ù5ˆVÜq‘^y"Ÿ‹FW4ıaÎ’èDÃ‚Ä9•ó±<A&H)¡lŞ^rUDG|…	åkÕ®G|åIÛÒ¦Ä?YqC°cÜ‹˜¡D€gØ—oÌXŞPVÉh¥tò¸)-†=·SñüzÄğ¼ƒ•Ñâ£`…Z¤!ü}yİËˆ†Í,à X>ûR!û>“ø4êûhOûªJRËÜ2f¿´ıÔ1] »ıjO!; GÄÎp+‘?ur¯™	üú…yÆ$=ÜC¿—¼á†xœk¾/²Á„ÙØ=µÄ-35'¥8,1§4U£(5­(µ8#$?;5O¡¸¤(3/]G¡89¿ ÊÓTĞš\À$69…ÉÄBIWI‡‹“3I•²	 Ù`f+( µ™›¹Œ ;)›à€Çxœ[Á}“³ 19;1=U!"Gk~biIÆÆŒ_Œ›U˜[˜ ¿‘„ä—zxœ›ÊÑÌ1á_hAJbIjhqjQ^bnêÄw†õ0¶BqIQf^ºBBVq~•R)TX)«–‹+­4/YA#]AUPjaijq‰¦‚{j‰[fjNJ1ª¼FrI…BjrF¾s~^IjP%º	
#ª¹8‹RKJ‹òÔPULŞÇ¨w¤•ĞD=·ü¢Ü°ÄœÒT„35u¸8k‰qjH¾o~JjH§§‹BiifŠ^h¨§Ğiù‰¥%`Y=¶b¸£„®B—œÜÄ¤Æ2dò:&u–]™‰â™ïL“8àœ‰Ì®ÎfC $•˜Ï®xœ340031QÈO,-ÉˆO­HN-(ÉÌÏÓKÏg˜œ1?ÁPpùzAÑÂ
R^*Š+ãç gf½à€Áxœ{#×)¶A‚qòb6éÍÓX¯q 5*÷§xœ340031QÈÊOÒKÏgH¹hÈµñÿÇğ©Ç7k¨K½Œ	¿_ÛcQQ_”ZRÄË½ª§B8éÀõ˜sy[®-á<è  ¹";ë Zxœ»Ây†“³ 19;1=U!"ÇDÖüÄÒ’ŒËuY ŒD	ğæœ3xœ›Ê4™‰· 19;1=U!?±´$c#?# Sù¥xœ340031QHLNN-./ÉÏNÍÓKÏgøb|híñt¾Ó˜Öİáúøz÷‹Sy†P¥¥%©y%™É‰%© ¥ŞrnìSv1×–	ıàKÁ‰MŞ#)Í/Ê¬J,ÉÌÏ‹OÎOkXgË´ƒû¼ò†ŞÓ=ºj3„6Ê2·C5$çd/J-È©´™å³7QóG‰Š½àÇ¥+
ÅAU¦%–T¤Æc·%ñØ¥…›9O\LÎ8pgî“¤|íP½™y%EùÅ©É% …wìt„“ÕİKMn~(íÙì]ÿoTa>Èh¸k"L$sŞÕ	lĞxÛ_=¿ Â*É.ª°(5­(µ8~ZS¤#œîné}pz=‡£SĞåSÿajósÀÌ÷Ü-ÂûüËA»¶¨úû~÷Ìy"¤¡JŠ“óÀjäªôµ-®:»ì­yÍµfÎ¢¹­¡jJ‹S‹@J„çëT™/:_òê|CÉyÉ•ü QÑ¹S±Œx¥UQoÛ6~–~ÅMè
)Ud´Û^ºùÁµÍ@æ`¶³>ƒËH´CD"Ššùï»#iYn6¬h¶$òîûî¾ïØ°âm9(Ö™ÛoT+ŒÒa(êFiqD…’†ïM„75ıØÅSU3!!Ú
sÛİd…ªG¥¸ç·L?°RŒ¶ê¼…Vç†×MÅ	£%«Fvÿ¨´Fµ*yÕ5êk"E ¨y&a¸éd±nàL÷ÌøE3i&EÁÛv¥î¸Œ³Ï1»p\S(*Á¥³×ìÂ>K¡k¹>}sOÚø¾š·3	È5…¶P‡Öh!·	Ä'± pŸÖJ'ğ£¼ã[¬+ƒ8[V¡d˜½]oÇ ›Ûd¶˜([ŞWûé»¿v×jOdR¢JÂ@l(0|3¦{
hn:-é6le–SŞMm˜¨x	Xü›|ú-|»‹lnŒø†f»ß;®JïÍ¹/ÖZ”0†¯±Lo[Zğç_¶éVğÇ§GJ¹l°&våÍfÓ„b"ëI‰ùo¹­-Â†”‚¸Òœº…Â²¤lù=«—/-•Ş$D8ŠßÖW„u2ŸöáçÄé€5—eL°]{	xXï©<„u5j£…J´‹¼jùÿä›-a~}y‰)?ák%Ã,Ñò È=p/­53ğÓæWïã$r•»Pè/n¸7Í/óUšCÃÈ0¸ï›åßı¼¸ú˜ÕÿÚÚ^Á¡­.f¾çEçCº ]‹"††iVc.->¢^úJ„ÁÚÉsfŸÑæØîJiI›eÙ¿ªË;Û “Nş Š¢#iUU78–,Ÿ±[kÔ&Xø1†=Ñ4Ê½/ğ…æXô‘ä;O;ˆtÃÀ×rbH¢4-²¹ÚÅI†Œíí´Ó¶%qoèÎÜÊ%§6abWA;A(Œ›Fes¾»¢a:0¸W»Sê0%ü„ÀIüÍ4´÷•uÖdåg†{,$êÏLÈQ'vÂ]h¡Èâƒ cø‚ÿÌæË|±‚Ù|uuÚ{ğàĞsŸıcÕ#EÑõPÿ˜\^çKˆ_¼NÑJx}‡×÷xı|@Ğ½»ÏÜôy6
Ö;qÕWQúR&_ÎµA?²EÑúùr|†Î8@™UQ_îÉ±ÚÏ}u,FŠ‡ÌAÿe±Ï4ÌPÌş”CĞnFŸ(=»ÆŒøŠT‡ÑÁ8}ja{Õ>Ö¯nMœüøéyô™ğpºƒkP3{È…Oá?
#Óê†:xœ;(Ø(´Á••UG!µ¨h²+«ãä3ìÖ›ÛX­™qÂ5â&xœkš!´!‚…×9?¯$µ¢D#¹¤BGas$Kãæc¬ZLhâÙ«N—²ŞxœµVKoã6>[¿bj,
)Ph$ÛS[A¤Î®í1ËH”ÍF’Úx±Èï’l*–Òín›C g†óø¾4~¤‚–z»`…P\ùÅóø®Rƒï†±È5Ûë!~&TÓªØH=eæ7“RHe¾6\oË‹İ(áü|KåšğÑFœïx,Å¹f»"£šĞ]Ê7hcïœ‰å9ü{ÑÈœf#k?J¬ƒÑN$Ì†¤ù½Àó>SiÂ ’rÇL©•xdù\èkQæ	B¼AÏÙªÔÈœ=ûÃJ´Q\hHÒ0èğí.Y§óæèß¬RAÏ˜ ú`†,×<ÆÄ!Ş²ø5·¨kÄ|¦O¼´ÌcğegòĞÉ åÃ¯,”–<ßàŸ9= N¨ab _mŠ®™·§w§Rì¬´A”
]…etMWğT2„Ó ñòÑ|Á¯cÖGgp½¸û½öwoı)øó·hÕÎÇğî[*Å³1“Á¤ôF2E–OÙ~vE¬Ë…xö÷ae‰Å3a_XÆ4·ñ¡U™iˆ(s’ñğ‹µ7 /Ì±==õÁÎclTâ™˜KüŸkr3¡%¨«Ú’M3}9Q]+&O„vÔDãÍ<5İŸ0B™5’ñ°Q7XeÏ’éRæF7ìaj½xmEtç¡Ğ–ñ=©à5' ØRÕ ÖFfèGæØ’€¬WSü?I‘®~w.6Àşø¢ÆoÈ/¢½fHÏª£)zÙºÌ¡š‹ÜªbM—×f“UTQë`~4°1#¥å©[_õÓÆs@ğ'ê¿ú‚Z«ñát­êâ¨{ª-gĞc‹{n¦Â»K˜Ìgà—€ZòîĞü¾YÂ|}{XŸ°$<%CÎ%‡ŠšàN;$¾ÎÊ*a¿Áä
m§Õç¤(HõygOªÿ'ù[2ã%@ŞZoK†nÄ£´¥-ØÙœÒ˜}}±h8Å5ùÃŒ*ÛøÚfÜ¥µ´
;,ÃV¶Ô)HD{—8Mç].‹„Ö²~Ü‡gºæŠñêŸt>„Ó‚‡ĞÅè:ü°.K'gûØ¶¨ä&øÔÌ\\.á¼ÆºÓÚ”í»À4cTšºZeeñµD[ıÖXœÉßâşëzregjkx½á¾gf_±ÏW{¿~U ãÇ4cÈl"5¢Üòƒ¥Ê7K›”ol½ïÚ$N™Zw—ËMúª¸‰íË |Œ0„ UJÄÁ˜ f,d¢à:í)£šà;Å-BµTZ’f«´„ÎZiÉ×¯°8®±˜LUå´^(ø€4r×ä!
VÏJ3&[I+4¨´}˜ap*öw¡5ö.ìØs¦Ş%¶æÈOìå£sIh×‘w}·¥ÃE,Yˆ,{@,úÁqiV‹ñ[Ö`’İgEÏ{ÂÁñ÷½¾Ä­÷K³PoÒ“ÇäCøÿ{ıèuÓı'ğvÕş‡±ûæÆèİ…ß‡ÜïoSDœ¾æˆxœ{$İ-=Á~¢gçDO-Ñ‚ìtı¢Ôâ‚ü¼âT%.N¥’ÌÜT¥É3Ÿhf¦è(”äg§æé($çd¦æ•ÄƒDJ‹S‹ÀŒÔŠ‚L ÆøÄ’ÍµLEŒœ0Cô&32ŠMfeéBˆlîg	æ ßN)|ìoxœë–n–ÙÏ(š\R¡œŸW’ZQ¢ç¡u&Ç3şã…ò4€*t6Ç1¥² 
M>Âv»öÍsÙeÑ×s‹M¾Àñ€#´8µ(/17uòVÎyhV„piaêz¬Csòb4%¼Õ XaIºËxœåVİoÛ6¶şŠ›ráHh÷ñ0À^¬dRe•“î1e$Zæ"“
IÅñ†üï;’’cC’»¦}« ïx¿»û‘%IïINAJ¯Z
Å´[ÏcëRH¾7fD“;¢h¨Š!şS)…Tøe7ÍÄš0ÃœéUu¤bfì®ˆÜ’Œ…¹8]³TŠSM×eA4×TrR„v˜YáZd´ØÕâk,jaâÔlM‡ŞÈó‰4‰„!DRNQCHöÑLğ3t}.*AŞà¨Æ\îAL7şğ@RT.4,êpÔë/z*™¤GÜ5
ŸõF"úÂ$ÑÛ…$\·ÌAn–àt¤m¦RŒç É.½eÅSğe	oä®F=¦ı´`”kx³×	Á™]£Y*%7¸¢ÆuØjÎ=Ì S}“ÌÇ RQRPZbH#ğ¶·Ü@#ø×‚}A9•Øı™ú4ÈôH86]í„Â· ô
ã-DÎRØŠ
Vä‘bIÎß&Ğô¨©Ì•ùîCÆ¡°—rWºX<şŸˆ7‘0S˜Ç‹(¹†‡ŠÊ-à<0¦´÷¤‡p
+o&ÖàÈ~°0êOÔOmz__Ú¹5vÔÑß²Ì%`?R‹xşm%Ù.¯[¢w¹4>>N/o¢ø'oÇpòßŸğıß_ğıõE/‰®o’x_€uòZÇÖŞ'‡dôDÓJ;$|Xp4€õ¦n9¯{%ƒùÌJ]ù—Œ™òRlb²pt‚ÅCñ4û=°h&bã7È61ó™‹Ù~´Ú&pÛ^O^ú!XØìÒrœ ¦ºK¸Øë¢÷¤´	¢rUè—–h÷±¸ûİzœ!›«Ø !ÂıÛLFËnÒ{„fØûöY(:ÉşltÈ÷PèÖ0ØÒğü0Î
Ã,}%¹ùµ”áXêæ‰(%RfÛ¡f4Ó–À|”+ÔÀÁ$…¤$Û‚Â]K‰b¶2^Vº‹'jtÅÉ.ƒ*GPeKml÷-ÅçT$ËÚ,ïvš4
xñÓú€x5é÷9óGËU“i3]'Â—Òù9Õéªİ».üşÕ&äÆ¾şÑetvıøÆ<çÉÕû.İ)üõG”D/°Ş'oaÏ\"ø÷nÇ]¯¥c©3Â- ]µ›ÿÎOÿh±FßqÔ+“	 î^ÖbEPVvÀ*Ç®¨û|„„ÎV4½ôäªæ3Sk‚€dÂØŸ5ğÿœ#-êÃ çÜ²Ãrw#·"ª¹qÚˆÌõ:ˆ±%GÁt‰÷oÿì£®Xú.¾.¢Ï±ág‚fâ†uxœÛ%ùN|‚ñDÛùçh‰d§ë¥äç§*qq*•dæ¦*m>Æ¨Î8™‘K|³³'L^o² £ÌäîN$‘·Ü=(ò"“{yt™¦!AíUxœ{'¾[b‚
Gr~^IjE‰ÒFÅ,F‘’
(_ÏBë(lîb<ÁÄåj$—T …‚Yş1aWÀ^Éˆ¦ø‡
3 €&­í€Ğxœk| ÀY˜œ˜ª?‘Ã”1¢­=k~biIÆÆ.FÆüÉÖì²“·3ZY³56g0™09sXdó7ïd±aŠ1±YB”ªÉ$ödFÉ—Ø7 U1çïcxœ{ Ğ"¸!Œ‰×9?¯$µ¢D#¹¤BGas8S?3šĞsÖ£Œ¨B“×²o Å ª³{xœ•UßoÚ0~ÿŠ[5MPÑDÛc%Z SGW(Ú#u7XqêØ6õß RH¥ö¡p¾ï~øîóGÉÅÏ$hîìr*K])«Í–1µ*µ±ĞaÑ‰Ğ…•{‚_Snù¯dR=çhû¨¡^qUÀI¦ìÒ=ÄB¯’T=©³%7[ª$Óg+%Œ>³rUæÜÊDa>Sğ<ññIê$+JJúDåS–Y•º¨ä›ÈLë,—‰s*=a]Æ’®¤Ë¬¨úË­ÒÅ K^né?i’dv)¿F À£Ñ+ïkf ®RE†¯av{Í]! cJ85»1vß­ÙvõpãAøìÈ•,,œ¾šm<ğg½ĞH…Y:{ƒ=ÆhÓ…,Âmİ:i¶pŞ‡{õßlt=Ü±J{ôJ/jËUÒ4ß…Oˆ_ŒL•‘Â.œQş@nJ<¨Üz³ºD¤‰¾’p1£s¹ ƒEÑÕôæçşh”¸Â@ôş¸O€ŠVààfòª…óóù|<„>8ì´ARâ
!]ì«¼6÷üı}4Áîrèşü.&Ã0N´¾İ3ıáf¿7O‹wG‚¨É=Üœî|Ô×oŠÁ%r8\1öaôšÖdÊYd3œm<{Î7ÃËØ¯pª×5Uˆ>=hVÛ'Oó’~&xái‹‰\nß>¤ø &––EÈß^Ç}9@Sãp“V7M§=–xvÜ5­™7Ÿ[£@ÅÛâŸyn’ÖO”huY¤é­À§>*§÷Òœôû4ßxdÌDãà+ï‹P*œ)ÛƒF}sÀ‰¶WÚDÙ¶‡Ù²Hä±gìn{,r=XĞH»â_ÜT²Ó6ÆxÄ ƒĞ™\mÕĞô—ÅBÇS€ĞÇüa‘„93´8Ø]‰½xÊ\Zy0	HıùÄ•?âo½­am)t\SZÆA52êU²Ét_%‡¨’w#hÕ® .µ¬ø¨{-ü†[Şñh#ÅÛ7*í´»=‘æ…ı/†p±Fx­‘Ño›0ÆŸã¿â†¦	*Úë¤¼¤!Zµ®S“M}œ¸‚°éÙ,CSÿ÷ù€®i³I­´0œÏß÷ù~­Ì÷²D0²sÕ[c•3Ô¡šÖƒPÌ‚B:¹“S{Wb6´®L#•† T®êvInš´P{5¯$õ²PiiæÊÉÌ6m-¦J;$-ët8Ÿƒ@Ú˜YôBí¾L	mk´Å@DB¤)¬ÑåÕy­P»e?®+ t¤ğZpB>lÃ®Ÿ¾¾«:«t	$°½¾·Î!¤ÎèÏ$¢¿k‡£š7±ŞC—„gGƒIÆ1 ‘¡~‰™ŞX¼îzø°€`›]fç_a=¬7_>O¹,Ü|Ì6øxxûŞgJîh<„§6‘˜y–¤6ñ]éç“lïêŸ«e2ØmÌ!| Ü.VQ²Í¥ß.É'ì½ºåèğÆ[ªšã?TğBIFte¼ªöf~ÒiîıÔG6Ü3ŞøÊ¸µétáUîısÜìM§Ú˜!f!qÿHö›EZöü>¡Úù"3åõUD5C>ıO–ÜiŸ¡äÚ	HVÑ²Á‘&ÿ²ä*“zN’í‹Ar÷`„Èò	ïrÿÇ‘_N‘C<0üÛL{…å‚\xœ{Èqs‚
Gr~^IjE‰ÒFEIF‘’
(_ÏBë(L¶dÜÈåi$—Tè(l~Í(Ã(™ŒMqiqjÑd{FÉÉ*Ìšb™)¼ÄÜT…‚Äââòü¢”É¡Ìq¨æMŞÂ¬¨äé¢£ R­
×áÀ4Şen ]â?3é€åCxœ;Ît˜i‚2k~biIÆD­;Æd^F+ zë	 ¿ßxœÕW[sâ6~Æ¿â,³“Ú[ÇÌöòÒ²à¤Ì°d–Ëö1«Ø4‹H"$ÓÍï9’m6IÓ¾´™!€tt.Ÿ¾ó³aÉ[plk–c¾‘Z©<O¬7Rğ½V;e†İ2Í;ú~ÕÆï\)©4}š¯¾Ù³}¹f"ƒöB˜åö6Jäº“Š;q¾dê‰¥¢³çk‘(ynøz³b†wDf¸ÊØªcÏwRë ³–)_•Nü7¤$Xó¶xŞSTO§±Rc>W\/§òg#i.å6K!Š"¯uj³®ğhÄw~;7C6Is²jMâÇP¼Ù±÷’{îlªÎï·\N¹á=–aŸø•âƒª„yÉê8`nšŒ!±ÖpËaáì18bˆá¯¸¹V=»X­7Jğ®QÆB‘-pµRHb^’/³äy‰n4àÕsJ!Ü2¾;<BCb§°ÕÖ;ÛÁäËĞ›o³|µªäpp*S?Y	øP!nÔ³k!úEhvf¸¢Ã<m=À,ÈÎ¡¤±äl€p¢,t(ğ§½»©zÂZ`ÎM²<Ì…Òæ¨ÆñWU?Óka§~ÙrŒõ[Ú“x÷¦ Ò\å7ô‘
µò›Éë»aeq—ãëÏEÀk¥áßãq¼wƒ$zÿ±íy-¼W‹Ü».dbgg€ÚM6ñi'ôÚm·	‰}‚?â\ŒúE>äñ'lÛgà+Í_±L`4És@ ©^µ‰i«£Éıê±ÿ)²ÇÇrç®
$0+‡¥Mô(­³*ÊdR[sliÚ!¶4­;2Ô–]÷ëSß²M‹ËØuöêó¾[)Wpª·ëğ&˜]½%Oî¨Çö}…-“D^j‰b4BD‚h6íáÿ‹9ö¸ßœ[àğÅ çú¬KVÆ © •(¤–ÛaŞÔ ŒK:7üş½Èï$B±I¼ m+9ªòÀ¹¢S}àÕ2¤ï¾ßûÈ÷iü}KŞâÑ›ĞÅj¢MüÈ¿ôÂ-:Ÿçún­*áVe´`ÓÚ³×²Ú[²ŠÔ,ps¬¨
æéıpñÈu¥ÿ»…PI"~ífxMèÃ+
–78‹ d¸mºoTş£I<âÛôúGÿ-Bxız1œÅğß±Óñõ3¾~Á×¯‡vãx:£«7Wéæxòv«¢à 8ºá²É¡©Ã¡¡½áDoC½±ƒˆha§&7pBl Qj Ihà”Ì@ƒÈX"4¹Îãg§ùÆ«ì¼çâá+[‰ôøù áÓ<ĞÒº|°9ê‚¿9Ûk!|wÜå‚3McşûÒNêºèÍ•\Ûåâéøÿ2­í 5¹~Ğ¨ıçƒÓzù/M¼¿Ñ^<÷c¥>rªT?ñ€¨ßĞ%µ&ÉGåÁ,®Ìì³ŸËdô¦yüZÊqáùµfıWº£få
‡Oxœû/}Oj‚ÉD»¥µD²Óõ‹R‹òóŠS•¸8•J2sS•6ë0M`åñK-×QH-*R°²U˜|€İn²=»èdY.‘ÉÿØ¯É&pqr¦å–èeæ•ääi¥¦ÉÉÏNÍÓÜ|Ÿ#˜Q†³(µ¤´(OY
lf^fÎf®'Lœ0‹õ&‹ğnFâçµ  :ä	€xœ»'uXz‚
Gr~^IjE‰ÒFÅƒŒ"%
P¾3„ÖQØüñ'#/”«‘\RºÍlÄ„&ÄÇ¦‚.dÌñ‚Q4«™Ò\‘èf–r¿d —3õì€àYxœ[ÄúE± 19;1=U!?±´$#(µ ¿8³$¿¨’‹+3· ¿¨DAc¢v+Xrb÷ŠÉŒ‹5&ÿ`\ ­>ÕîGxœ{Ç2‹u‚
Gr~^IjE‰ÒDÅÿ¢É%
P¾3„ÖQ˜¬À¸˜ÊÓ ª Ša\ Ûfå€àxœûÊÿŒŸ³ 19;1=U!#Ç=fFÍŸ™§1+k–¦Uê(¥Å%©)ÁÉù©šzÁÉ‰yjÉù¥y%“YD'e7  zZ‹éRxœ{ÆÿÃsf^çü¼’ÔŠä’
…Í/˜§1K¡–¦Uê('ç¤OÂ #Òè	€ÚVxœÛ§<U~‚ñD—Icäy
²Óõ‹R‹òóŠS'7ÚÜÅ|–‰& ·9œ­‰;¹WlòK^$‘@FáÍœœ§‘ô\æZªGMı:ŞíÈ
ê™$7×	80 è€6üêdxœ›*ß 8A…#9?¯$µ¢Di£"#ãäÙŒš¢É%
PA=g­£0ù£Ád[¦ÒÉ'˜œx¡¢@•@næÉÇ™gMÆ¢Cï^§Í&¬÷˜P5næ˜Ëˆ&´‘Ó™Q
«!¥Å©E
“²ˆlæåşƒ¦m2ïMBºşğZ¡ëâfVÅo=y;¯¬R@bqqy~QŠvcJ3S&ssËs@•MVä“
Óç—_œZ2ù'·îä&>ûÉ<¼“ÏñóÁÔC°ç™Ï+ „êªxœ31 …´ÌŠ’Ò¢Ôb†/»DùM“7^ïïn¸Y®Õ¥<£a±	XIf^IjzQbIf~^1ƒhCaa»¨æß×ÚßKîf¬ı®¿ )M‚¨xœ340031QÈO,-ÉˆÏÌ+IM/J,ÉÌÏ‹OË¬()-JÕKÏgøzV[ÈäãÚÕ¬ŸyúUUï¨  OtêÑxœû'ùRbÃzFÖüÄÒ’ŒÉ{ı¡,)¦(«)ÊzÎ•İìÂ\Ë¤–™¬£™—V”è’_§£ZT¤`e«éœŸW’˜™—Z¤ç—Zîé<ù$›åæ¶26 /R&
à
cxœ{)qQrÃÖúäü¼’ÄÌ¼Ô"+[…LgOÏÎ¬®åâÌLÖQÈÌK+JtÉ/ÏÓQH-+OFUZQ¢‘\R¡©ÇÅÉééR©¡	”	È/.I/J-sÜ‹
’Á×äŒ| C¬Ö;1-;ÈóK-÷tÖØ¼u5 ¥ç4· xœ340031QH.JM,IÏO,-Éˆ/I-.ÑKÏg¸¸i•ó„Ë¶{e\$ûjÎù¥ ¥¿Æ©0x340031QHÎHÌKO/H,..Ï/JÑKÏgÈÿ»JâîƒL¯/"&ûtüëø8»2BUçd¦æ•Ä—§&'§‚q][³™Kïìù—S&œévè˜Uœ–_”_‚btˆã›i{Î÷Ï+ØøÛÕ¤+ÀÂ­t	TuzQ"ĞäÄääÔââø’üìÔ<ñÉB¢l¬Ş9kä­XÖŞ9ÎôËíî]%•©ñ‰¥%ùE™U‰%™ùyñÉù)`§yßÒŠºiV€§pHğÜ’Øá§É˜z“!^J.JMz-31§d¯Á2sÇªgÌY¢~øj&ßÎsüÃÔ‹jç7»şÄ@”;zS*«áÂ#çT§®ÂÔQ”šV”Zœğàìy³¾M/L›ÖñÕëı¾ùÑ;ÿ‚jËÌ+)Ê/.HM.¹èpà6õ^ûx¯|öïµ{¹ÕñØn¨ÂœüôLphİ[¢ğáû$#§#»Ş0ooßp¢›ıTM>(Œ#Î>SáæuíûÜ½·Î:wg^CÎ{¨ZGÚ	·8UÆ”«Ëgaª~‚c¯óD¸ÚôÌâ’Ô"OêONnh84WÃ)/JÁš÷ãí&˜²üœTpÀz¾¬şók¹Õ‘÷IßŠ7Ï¬ş/iUSœœŒK¤´e}±ås³ıÎU«Ù†ò>‹`j©× [›¶Xİâ÷J<<çò¬şO¯2…ösAUÍ**F61éúş§ÔËRÚšy{sîS í×8¿äƒHxœt ‹ÿ‰‰âVßu‰z {á‚b CU©Ê‘öº8åø×æ”Vl@{À‹TtUï–@'100644 login.go ˆÖÑjËˆ\Å£›»nJ†²pëÅ“èk¥U¦W»CÁO¥_ÏV×g¡œ“g¢¿m4Ëè€Ïxœ{Ìq‡cÂ6çŒÄ¼ôÔ‰¿t ¬É:Œ20¦c2Œ¹qŒÌ”49ˆ®è:“şdff¿ÔòÉÌz0áæN“™.jÎ¢ ”'şë€Ê}xœ{Áñ}‚ÎÄz­‰	~kõ¹ô‹R‹òóŠS•&'2^à„ñô&2E#x›{˜î0 ³zç
À6xœ;(xšY½ 19;1=U!?±´$#´8Õ9±8•‹+3· ¿¨DAƒ‹S)9?¯$µb¢a„ˆ&WZi^²‚Fi²‚V)DéÄó-““S‹‹Cò³SótR‹Š¬lJ“õŠRò‹3Kò‹*õÜ‹óJÊ4’K*t&§±*L¾Í(?ù?;‡TQjIiQŠYy™9\µ\ ?:ıà€xœ;Í¼m‚öDsm¶äü¼´Ìô‰áœœù‰¥%.%ùÍlXKò•¸8òÌÙékgˆ•äë9&'§‡äg§æ¥äç§NVcìªçâLÄ”ÑQH-*R°²U€ªç—ZÅ.Ndí:@n^fˆ‚8LÏ)±8ÕÂt,(Ğƒ0ıA†êAÈ$S}2ÓRK2s¶ëë+¤Vd¥úAÏ)5±(µh°&gQjIiQVgmçªå <`bÖì1°+xœmRÍŠAfÅ™Nö »‚"H‘‹™;¸şÁ‚B2Q–UüOÂĞ[“i’t==¸«ˆø "¡oêA|½úúŞ|«3Y6ÙİK×OwÕ÷}]õëâ‡õ­<ãd„ “Òf/
Œ““Ó\m¶„V÷m‹ÜtêÍìËÕ€’©Í¶¿Ï¾v‚|<ªÒ/Éë,r­
ôVNÉFŒ¥¥Ğ.tÊ
"‚>ái#ß$Vjë=|heÛÂîÃ’Ç•í‚»S¿wÅjŞ‹â¹£zº€é£MoYØëÁuî>×{nmç7°}JÁæºV›ş ­ÈN€üéÑ» &¡
½ç~¼±Å!ÎPŒA¦îSãUx¨»ËgnpÇšz)ynmÓ}l®ß'679ìèH6C é†…É‘†.L©0[DG|©J*ÏÆı¬mÖbOÈı¨?K2”ºäš÷7¸Şâ0Ä	ZtßêÑ.}‹ãŠ«'$·iœüYn$ızrü’?FîOpíımo0±8—P‘ëùÂ¡äM+sñjæ‹5¤áíâëSæG+¶ò%.ÊŸ¨¶hoâÊíç9¯ÜÇ¾=¯Î¥ö;2E¿}¾œ¶’01h("Q–A[§ÒVrÂŞ±ÿ"oâ‚nxœkämãİ ÉÊš\R¡£°Y’5ƒ 0åÊµyx¥TËnÛ0<‹_±ÑÉé^ ‡Äiƒ$iïµ’	K¤JRMŒ¢ÿŞåC±œ¸@^,‹Ô>fvf.¶¼EĞ|t›oWÜ"c²´q°`Y.´røêrúÛJ·ŸK¡ûª–[y±áfÇkYµú¢—Âè‡ıĞq‡5²¥˜öF÷\*8!^RM£xW…øª	ª^×Ø½%uú2:}¢aëáœ *ƒvĞÊbÎ–Œ5£°œ‘æ%¬:‰Ê­Öô¼³·†+·îíå*Ò_€z@°ÎHÕ`°¡ä›'½Eõv(B:8Ÿ‘^ÆKX¤S§Ë+!ĞÚú, Ñf	¿XVUğØ€ÛàTœ/Ä2:z˜UağéFQ´•N›]y‹î;ïd=ÿt1ï¸€Øê’e²ñ¥áì”ì|ùÌ ò¯!=Ë~³Ğe-"X±P¿E7¯÷èéòL>h~“¨<©önì¡á–ñ=‡(|9,àEV(P*²Qæ;¡Ç»^Òxİ?8YvR¿$)²!M/êï ßƒ™û6£\Hk|9¢ßı.½~ LšIó”’ÅP^Ó>!Ó6(¯†ÁË™şŞûeSÆßYÅ;Ù “=zØdŸò¹	$ü;ö$Y¿{¸¤(FZ:âÃ¿iç£Êè»£*Àà­Ã:è09”\8ùwo·ŸÜø=ç—¡B£‰İ>vd ½÷ h#I]¼ëvĞúÅ5<ïÂ=^FĞZQ^®™W$D“{)C}mf¹¤; ¤,Iê	DÄÏìÓˆ!X÷LrqGÕkÙV4çaô‘0­„<OVÂ´&¢Ó‚\]­×÷O„@tcÀÕ.](íPœy±=\àv²—~laÏ®µ»N2‹h>ÿ8°3çN¦+?óÖ]@¼âŠº¸Æ”3bHŠMù£Fÿ KG•¸â„bxœ›Ê?‹C&krI…ÂæL&V 8|X¼Pxœ¥TËÛ0<[_Áæd‰|/C6mİ¢PeÆbK%ïÅş{)É›G,ºíÅrÈ™¡<(½W-‚ScØ}÷¸Q…0ıà(@)Š™v6àC˜ñ‰ùxjMØ?¤v}İ˜½Yî=ªÆÔ­[öF“[ì‡N¬¹|kZ®IŞ¹^¯¨7Ü¬êêT_7	 î]ƒİ4¸ÿAîuŒ†=Ó©„¸SªkxOôÑŞ©Î4, C÷xKŸ•÷÷RŠâÅŒdaå'¼/gSŒS"8‚aJU±ñv´ÊQÃ|Ì~UğŒuMÊ†R‡˜l“›ü^ğ4ğŒmàµğéÎ 0?1LnÒ·
Êù³âr­5zÿÍíÑ~A?8ë:©àg’å„Á‹"E)Ş®`Ô’Ó¾Æoqìi–Jf›RŞ¬Àš.„a$ÃT-Š'‘Ú¬y Îh6'õ‹LEŸ§mÖy¿©üSˆ‹íùKïQdÂ#*Ï$0Í¡ E‹¤ºLzVƒ/Òoƒ¾düÁkoÅqvuÔ6öÛúİ	q•±Y´lYöø_$Ü09&Ï837¶>l ¯ë…%à+q6j{Â€ãüOW¼º›|\ƒÌÇÛˆ.óóıÆl16ËùÊ+TÄ\Eñ÷<Ÿ}»D3Å“ø«¸,ïƒxœëá:Ã9Agâ	­‰	×'³1êsé¥äç§*m^Ï¸‘ÆÕÛÏ¼– ­ıƒâ6xœ;Ãy‘sÃJ&Öä’
…Í+™”™ C5.±Ix¥TMoœ0=ã_1Í	"ÖÜ+ål”\¢VÚ~Ü]3€µ`[ö nTå¿wlhÃF9tÕà™yïÍ¼ÒGÕ#85Óğ-â^EÂLŞ‚RWÚYÂ]ñgoh˜Hí¦¦5G³TxV­iz·›ŒnG8ùQ6Ô™crÚ{7)cá‚xÃ5ƒUc“ã›6'h&×âø7)¹ÿÉHî2FşÈt*!ºÙj(g×ó"VìÆá«;¢}ÊR©é«nr¿èW¥ó¢v!R0¶¯A-ÁõF'¹Ï{”ë.9y«5Æ˜+0zg#Ö€!¸PÁ/Q4< éh@œ¥(xk0‡ÁÇ˜µè]4äÂ³|Dú®FÓn¯–+âc%
Ó¥šğá¬Sİ" ÍÁ¦eÎ+Š‘ÑpºŒ%“E~¶¥ùÂ¶Ô—$J’ez‹7_TıÉõÀ~Kbp“‚(Ô«|õ}²˜[HelÆÀÄŞ X{R¿sÂ3ÒşÊQAİäyaLKSÏ u:µl1	Ûáş|Ç<­gLy¹zaáÊëe.å9û’GTŞzŸ,ÊŸŸÓ@.ÏMö'Ó!™‰ıÆ.Ã“7Œ•3ñ<È;T!“ÿwÊ«a6yeÉ./â7‹áŒHâ‚]xœ›È9sƒkrI…Âf¦R& 2¦ğ¼ÙxíVÁn1=³_ár¨Øˆ˜;IK*¥RBÚcå˜,v×+¯7Eù÷ÎØ^X A¡Tª*…‹×3~óŞÌ3¹1¦Eiç÷\ˆ¢H¥¹6–µ¢FSêÌÂÒ6ñq¦ì¼|àR§‰Z¨ó¹0+1Q™>O•4úÜBš'ÂBƒ¦j†1.íP§Beìˆx…gšL$ß™¸TO Y'µú”ŒVWQ¾ r(ˆ†ÂŠìHò(Ò@‘c0Ğ™V¥øGÑ´Ì$k•’•^¨˜2kt‘ƒ´c½€¬%í’Åø…W®Í,½
_ãU_‘^VX£²Y›ÉDş<«)Å/ÜZÌZaÕj¾9è6@k30F›˜=G5õ‡¬³÷z¬Ù¤7wøfĞ9Nx_J(
› E—(jÈD¨´p©Y·ç»rˆÇÿ‰š`gù*CA˜
»Œ°e±Vzìç9•ß©ŸùõÏñH6v0ûÔc™J4\/MÆ>ûÒ_-ò¹/­z„.›Š¤€—6Åz¤T2
ÃoôS+æı)6lË£ç—Ë\¡€}ËÇ¸ÃÔ8é¬âIY9ß¡É•Xù›„v‘|09âH" )÷¿^¦?ˆº5>¡;d-^)¹\Êj³â}ÔMIT‘Ú4ô#
C(w”9M ÜÀÓ~ç^ÖºÏ£ÙªÀ“†#·Cñ-LQØ¹«“šöÇÆïı‹$‡Œ‡XşÖJhËÉ\Mú¿"|‘Ó»œå˜Š2qÜ!qèÚ¬²D~iÌZ€QöHÎ@#‰öñŠA¾·ö]³ÖÛîXk"•«Ú€â ]¸Õctíã•½’ıt&œÊœcÖ”ĞÆ¥;©s´+úxhÜ­Ğ+‡Œ, ËğŠá½XûU—aOï»Ø}¦–­8¦­£¢(aÒ·xBmkµÊk;¯Ç#cd4ÄdòWòÍhèLÉ‘¿Ş0‘+@ôAƒU\®~îp×Kh7XV.ĞŠøpû!¨^¸ù7Xí"Ç¿Jf7^Éæj
¬|˜wœ _:4eÉD
ºÄG^ı~½Ä»òÁUG½ë¶†¾À0pû¬ø›ïÈÛùı!«{@5DîÏ¯Çş¯cÖå…;àcÖ>fgóİ³ö£œë…^xœ›#½ZzÃVÖä’
…ÍOX2±€Y<9Œ›¸‘¸ LŸV±JxœÍRÉÛ0=[_ÁÉ¥öÀ±ïæfÚ"À )ÒöT™¶…Ø’AÉYPÌ¿—²\ÀöÒ[/Z¸<ò=rê,+Gß~w¸—…Ğı`ÉC*’²ÆãÍoøÙhß?
eû²Òg½m%İe¥ËÆn{­Èn=öC'=–œTë†s&ØgÛKmàò5×$#»rÊ/«	 ìm…İFdB”%¼Ø†1!Ç;¤Rèx{Fş˜
kB×Î–ÚH¤m´º/[şi“‰z4
ÒQÁã%È"~ªüfŠ}¼sPFãáqA®ØO¶<XyXTr98eçI›&ƒt²›ZÿúÌ×É§Èav!‘¥~Š„8¡ÉDèzªüÎÙA;0Öƒì:{ÅŠ^ğ-Û9ê¢Š„SFUÜ‰3v12 E0‹¯s¯\.ÔûÄeª‘´¿³¶ÒYÃ´(v!¡Aƒ$»¹ŸùğZqæ`t—Çã#ÑÁ\d§« Œ‘=é‹tîj©É«˜Èí§¹2¬Áëj²"‘K±¸¼æA8X§½¥{ñ™¤ñMy“8³œ_b¸ãxÚûøÜCŸÇ0‚" ]£×=æÀ-âmĞ<Ş›¦Ë¨Ù¤kèêá)ĞÄûCö¿eÊšqi¼Ö«Õ	½İ‚¿pF¤ˆµÜ™ÿ—úïÅYNsÍ“ãÅ«øş“¤äƒxœ[ÈùŠcÂæ‰Z:<ÙéúE©ÅùyÅ©7²1NşÅ(Ç¦ç—˜›ª99YŒ&§7¹ˆ©œ¹¤Bgòæ™›ë˜•e¥WoKxœ{ÅñcÃJæÉ«˜æNvbÑ  8"ç€õxœ»ÃvmB;k~biIÆD£‰<!ìÉŒÚP–ã|(ëc¶˜å’Ÿ›˜™§ç˜–P”ŸRšœZ¤Ã¥© QVÉô ¡2 œ»Axœ’AnÛ0E×â)&ZIMÀ‹Ö)‚n\ I@S#™°Dª#²Qäî’j &R@€éÏç¼?3)}V=‚SÁŸ¾Ï¸W3
aÆÉ‘‡J¥vÖã³/ùˆDf>%ñ•±PöÆŸÂQj76­9›íIÑUµ¦éİv4šÜÖã8ÊccØ‰¬šTß´É ]‹C´ÿ£éÜ—¢â§¢ØcÓÀ¢üpöØ>j71‡µÎÆ{B® RŠâCÕ2£<à¥*_µ0G1è¤†#BŸõe»àëïÑ?`G8ŸÜmòBÈÎKqçX¼ğ÷¤¥¢VC4Ü†œ~ıO·JûgXf!÷ù½ùc•dp»‹\DİšfOÆö5Tù°ÉÔ5üÅi‘›ŞıuƒÌÕÌÛb§Âàü	BG¦7<ŞáÊù(s;^Óvp4K.‰İù>€|«(8Â8Â¯İÊËpljÓvá-Ë.	b=ú•43ˆÂto1ovP–*Ó$LF
ZNn6ŞÑUrØ¯¿ªæR¶eìeÍÌŠ<XöNüåEğ“ Ş®ÌşÓáğí‰	ôZe¯Ë¸HïK7¼ŞòqRïp0£‰İ$ÆƒóË¢VĞûé¤!®;ühçsëKÁbË¤âEü£Çhóï‚mxœ›Íq}‚öDÓVFÎ‰Ú¼ÙéúE©ÅùyÅ©J›í1qÂ¸z“2[ ƒ*ƒè€ë9xœ»Á|…™³ 19;1=U!"Gk~biIÆÄ{Æü‰«¥ó'72ŠL¼Ù fˆNÎc³Â8e<ç€ê\xœ›Ëñ—]µ 19;1=U!?±´$#´8Õ9±8•‹+3· ¿¨DA)µ¨(¿¨Xi£Éf–¢üœÔÉO™- 5’"íDxœûË>c‚¬„§RjQQ~Q±U\R”™—djnTÃ,
åê…äûä—§iåç¤jN>Êl V*Dî€èxœÛË¼Œy‚yzfIFi’^r~®~Jfv¦nFbQebJ¦~z¾nnfrQ¾nIjnANbIª~Avº~QjqA~^qªÒÆfFiOÏµ¨È3¯,1'3%89¿ •«– ~<"Â±æxœÅWKoÛF>“¿b¢CJ2e¤7.`8M¶ƒÈiA`¬È¡´5¹Ëî.¥…ÿ{g¤HYN\ H}°¥İy}óÍÌ–ß³‚d­YÔxÉ4Æ1¯©$q4A‘Ë‚‹Õl%—û])©´ı´âfİ.³\Ö³‚ßóÓ5S;Vp<­y®ä©Áº©˜ÁY.EÉW¤ã¼¼–5ãş…>•`ÕÌéÏ
g`VË«ƒ@VRñªb3Zs)\œÍlmL3‰Ó8Ş0eaÍf°0Rò…—¼f5B–eqtäüœ‚•wÎù«;jƒê.x˜8[”8~Ç·spvÌFK½ß”“%Ò,SoåØÅ9x²kÜ&“à„4 ½Ä$µPÍ®AX'#İ¨67ğw…À-L€.S™;è¯ocOá¤¿'½„Ÿ½Dˆ$Œ~Nlö³øW‹ÚÄÑv|.uC&ğ“âDuüÇe+rà‚›$µQS>àŠkº·P0ÃÄR*‚çH#…PQÅfRB*É ü²=ij’Êg—/…¦U„ßûêN¹ ‹}t{ñ$%œø2Ï.İŸ)Œ=Ns
'IÂæ}ÁËpD'®>Cšs)ï9vénhw>ò2kÊ±RçğòUë3zÏÌzî8!8ÙEÓ(76m»™•²¢+öõb…óïˆ{)§ğ˜¾ÕnşM…w··ï­”Uy _œš Ò×4fmÉr¥LôyXI@i°]Íj¯êùKtOCúØ|¢Æ…;…í±ZuÕ©3E]©ì‡-}Øv!ÛvìØp½ÙTˆ'ƒÛ5×P£YËê–0,rVUÔæK,måHŠ_YkM»¬x„5È’‘rèl¦›}Jº‘Ù#—Ë?17Ç€‚¤^s£e0'¦öæç ³a©eoÑ${zd¦qÄK§öâ¯¬µ®ÈéÔRõÖ(]ºŸ^†TBÉÉ Oû¦´ íìì5£k®ää‰îŸzÈİ”¹b÷T.-%~ÄŞšò»D‡p€áNú:=6Õ-ø0ÍŒâ¸!~[5ÓJÅlçÛ)Ótkˆİ8j‡aËû-Ù¬¢:ı<~w¾dO÷4½ 3‡qŸ—ÃĞÜ¼õñØ¯N‡)…¸gr1fR³>‹Ç±^2°OâÚ×ğÇç·¨¤C[ÀDß£†J€§Ïà^mAYòMFs¥{¤.+dj˜KÚ‚Ğ</›‡ºÉÏÕkìãX#"9ÌÙô`·JŸ›&"åMÅôúŠ„ìfà&#ƒÒÑu‡S+Ha”må¶ˆ‚kZ8w´íR÷…Ì°Ò>#?Ÿ½¢éSpE3ôÉBúKj½²L¶~x/ŠÂ%x„uX$àbµA>;¡o:Æ%WôtıÉ£ÑPvÏTÉhéyøŸæ0å P` T²öÛƒ¡ÿ’œ+'@½6¼oüa’şŠ$È¤ğ+œAXæ>!ÍzÓt£o¸“Lı¿å³[)~rKÂZnlÜÑ“<õ CŸÏ¾øÜa½–cëÚîË€ÃœíG÷e%µÛ?j ±Õ
a[$ìRzÏtëÿL½Š]Üâ åâˆ3xœ{(³[fÂù‰nò<ÙéúÉùyÅ%‰y%ÍõÀü¢Ôâ PêÄ›o3¦³pÂèmvb“fä„ÉëMe/E’¼Ä†*)3Ù…S¡`r.çs4ù.#q¸|hqjQpjqqf~wjådFÍÉ7¹î£êØ|…[Í’ÍŒ¼	Œ K¸è
€ğMxœû,Ô-(W˜œ˜ªŸXZ’ZœêœXœÊÅ•™[_T¢ 1QÃ‰,3±âèä F—É³3'+0g ±ÔäLÙHlÉë™&ÿçœ,ËâÈRZœZ4y.³ìä/˜àd'V¿ÉŠLV“ï³:nVe“aœìÆ>(i<Y#òl ;V¤SGDæ%æ¦jrÕr w9ç€xœë|-4AS)9?¯$µ¢Di££hrI…T@ÏBë(LÖbtaÊ YyŒ™8ÔØ3IM¾ÇäUgÏÔJ¤:¥É~Ì8Ôö1GAÕ½`Î&¨f-‹Ÿ05“ï2ÚL6eu„¨Ú\Æ*ÃÕ0‰m&NC%'›rHMcÏ‡ª=ËîW­‹8X]iqj„ÌKÌMÕäªå ryUµxœÌ1…  Ğù÷ÄIä¤–ŠV)‰æÇ»k\ßğ
Ò†‰¢ ¢%Ws][Û<RÖe¿b½0JHÙ«PÍŞXËÆ4v K;è+úÁıá÷â8LÍ¸à†$"G¯x340031QHÎÏ+.IÌ+ÑKÏgxo-¾%ÏäšÓ…N+¶i•ÏÏ¼¤Éab pU³5J¶›šjĞMVØ|ûpˆÇI­ˆ’Ô¢¢ü"óÇlÏv©dİQÛëjwù›ÉöÓ"W!6$—ç¥€l:œªP“&i·ñ‹²Z’á!¥;§kBŒ)J-. :)•á&Ëá«i7¾Yˆ,‘.Zùiú®­—BM*.Ì²ÉmëDËı½kŸ	¨YóF„ÿÇ±u0%E™yé E¡™â¥Ş+×Ü‰w«k—N×æ?ÕëUTZ’™SRsğÔÿ×òû6Ë:ëìÍ¶ê³>9ñ	# ñrŞîxœ áÿŸŸ ¾ƒ©Y#İ¯¡«V÷0ë¨~y¥(¸‘´kßyµxœ+HLÎNLOU(ÈNçâJÎÏ+.QpJM,J-R°UP‚°”¸ ğj÷«xœ340075UHÎÏ+.IÌ+Ñ-IÍ-ĞKÏgğ3É²—JT°ÌThtóh´ı‘†f&&pµ eWÖ¸Nu\äyzÓ‰çaÙ.íø§:ıˆ‰(¤å1l^ú4XıÿÊcøŒJÿ¾˜pcê>1FCˆ•%%¥%™9 cìO°œ½<[…]ögGSXâÆÙÓŸÒ…““ŸZÄbøyó/ó7bt°x¥»D¼] iÖMÅ³bxœSAnÛ0<‹¯X R$rƒ£­›p“=ÓÔZf-K*I%Šü½KQ²¨X.¬“=³Ãî,+.v<Ce¡/c¦©B¬¸ÖI•Jší´Q²ÈæwÃï…cSV<¢.k%P?Û3ö€Íb>HîËÂàC¨‡q±Åò‹U{BÄØWÖĞPMe`ìëfëŸ±`9¿ú
öé±‰ÊÊb‚å'şéK2©Ö[>#KËæéû7h?=µÑ¿sÒ­Jm2…–uDå mUŸx®†9İÂU0¯m,jOĞA²ZÀşÑ&^çÜ„©qê Û÷>çrŞ°‰³ä*©ÍÖİæ0};Bã¸&ş²–i|AwË[oGÕª7ÇŠŠ:Il(¾HIúõçÃJÉnZ…/ıõº‹ãÊq—;l\+Û¦¿éÉV‡ãiº¸ç9B·-íl—€º¶´©aÅÇáFğÔî[u‹Y ĞÔªè ’Dì­×§k
øQºöDrÈ9‚EaHG[í‰è_(=ÁÎåyFŸ§Rf¼"§{)ó_ÙDWı*Ø‚2Ü™	N[mÓrPÌ‚¾ÅlOè¬« Õ=.à•$¾ÛUŸ ‚ßŞz_ÖVbJêÎRôî1õÁšsán½öœ5àQdrÊÎ•o&Æ{Lgd#ü)gİ:%›¬>Eö.
÷g½iñ>ådMW1H—Ç?şHéc²xœ]ÁjÄ †Ïñ)ÄSKY6KÛcK»d6%Ğ¼€ÕiVjQ‘Ò§¯ºÙ¤t.3|ÿü¿ƒ–‹/>h|à&²İÒ½µ¤€<½ñ	hCY‹›N	‡\T6LVó ŒÌ»w¤Jë{‡2;lêì_!Ò%Ä…l‰…Ô¹/oC°ô¶ÎŠ57Ã#¦¹Trh\_0ÛŠØ£[Ä§º®Iul”=Cè3!U-1ÿ2/	}Ìæë%}G~=£;şı‚ÆĞ[5t7ã“Ô°H+>«OÔt•füîu‡ş„0©<ÿĞ?áL‰t'å€oxœ›Ä|eÃ$Æz®äü¼â.NÇääÔââüìÔ<Ì¼[¥D°X|	HP‰‹3(5­(µ8¡¨¤"S£É…01¸$¿(1=5hDf~_bn*HCz~|~biI†Q|qj‘mYj+ Z
‚*÷N­T t ¥@d åhBĞ¯xœ31 …Ô¢¢ü¢øœÌâ–ÿ<]ßíÚOu}uíz”)¥ğ0Íë“¡™‰	TUIfINª^z>CÁsëıwM"šN6®ï~u‘àÛ_š• æş!÷©xœ340031QH-*Ê/ŠÏÉ,.ÑKÏg0ûéZ™ôö §ÏÁ÷wÓSNÄ  3³›±9xœuRÍOÂ0?¯Å£äÃ[â$&¨$ïuë q¶³ëˆÄğ¿Ûö1ÆÊØaKßïëõíå4ş¢L)©–¼Ğ„ì¨‚g¡™4›We¸ç~‰½Ï\Ô¡ĞªŒ5ü‘àƒf<¡šKáP°O­ª”w¦vL!å~•z!K‘ÔÚ†ú–z;ÿYn
cíÄ9{lëí¥Ø€=r±!ÁL&î&'EÓèL6å"1¿Y£KK®oï,ñr„Ops1,Cú}èÁ0Cˆ`*yÆÔ*£Úµ”RFH™•…–ß…S`'Èc†Ì‡¶Ïåc”¯·\%½œ*½G8D8Dx1YO–Ä -(‚Fóv@§ºŠÕéZ†nä®h‡®5ö6ãŠ©b?%3“ÛØR3›Ä3ÖÆ¥¹b+¤†Ô’<›Qmã-Wä-„s;_?©ŠúDN£ícàã1Ğ&âë`Öèoú·#xœm»RÄ0EkòšTğ<63lÁ«± ^+h
¾­•oXW9çê5`ÿ‰ï¤*z#iŸ1å¦é?8oÎ6ª×èk¤}~â	–w	­1˜`[d+ñ¹_«ElÍÜßJîdLá”o
tõ9á˜?Dù‡uSkæv'úÊ!PZW6{a®.ÛíHÆzrS'ôÀ]Ê¤	ã#é7iÙÆS˜8¬w½•ò¿y¼3¸—@±Î\ƒ3Ë*h™Š•nÜÓq”Oôš£\«/9”Br¬Îkê‡msÑü‚>®d¼mxœ•”]o›0†¯Ç¯°¸j¥jën‘vA‰Ó¡tĞñÑi‹"ä‚— &6³M>Võ¿ÏL¦.Ğû>ö9ç=T(AKrJ¸@DXV¹©(àÊzg,>¬„¨lëÚ²…ú$jî‰ıÀàùnµ ®ós~&PÀ÷­$œi1dŒ²)e›qñ*"ü«Æ\œ˜RòBèŒ›î‘À;tĞ&ŸÌZ7æË&-‰1ÛbÖµ;%¨+ÊÊß¸9Ò”ôÍbXŞÂpœÛ:‰v¸²yÕ™åÄ¡%ÚPÊ›1né%Ú#[ş\&ã^rìà­ËÂ•UJR™#QRrêydtP5±’™wSZ“¿•¥%*s[Ä o¾~A•”mP5/‰XÌ\°’,_as~µãÄMÒ8gö°;•ıvs–;ç²ïÎdüšÂ8Q€®tP  kÓ8¤utFh÷n¿¹ß-¤œn°XÉZÀNnÇ(YÈ“¸:ÒîCÃè	FŒ¢0úWª™ÑKM7M>‡‘ÿNSÎÁ v¹e —kRæØÀwY†AÙ…†aNk–c°B<cÄ¼58tgçz|å¡Îm û5™zf^(û$—‘^caÜïÈpFwşdÅ;îœÑD¦9™ı…EsT?xrüI¦f%ïê{nâ‡Í‰I”aÄÈŸ`»°¼ù»ÉZH©FÚ8†Ö»8Öù LdmiĞ‡¤Q+À›eı¬Iº@Ğ_åòäö^ƒvyÜ^†EÍÈqÇçJ´˜ß.€ïÅÿ >*À›X)Ü¥xœ340031QÈÉOOO-ÒKÏg(/xv}í/§ ã‡˜fw&p•Ú_·ó şØâ»xœUAnÂ0E×ñ)F^µ‡èÂ²G`aHjOTe…"„² ‚
¸¿:c'Äd5ïıŸ‘í¿ñt§3üŞ¦é|··ëã9^ŸJd‚Õlbgà´LZ5[¢®™Xü´q‡QD™´RÍiÛ:È¹{0{xyAÛÆ2k;#'¿{L´ôgÌA†uQFÖäßöfK>`e)ˆNdìîHÑX]!‡¶uõ&ÁòõéeÊÒª6#KÇ÷ğ!-rF0ÆºÍ(‡º·£3®pô®~ FÎúèë5Œ,ƒ!<Øa‘3jõ©şû”jx¯
xœ31 …äü¼’¢Ää’b†EÇ??Í±yñ¤²MnÃ‚*M‰{7l6¨)-.ÉÏO-*Ê/b¸Ç/°ÛJ0i™{ÅDåÏn2ü'ì®C”åãKK2sŠ®[M¾x#S ÿ«Öw¹×ìˆ>Îñ	¢*½¨ ™!¢Òmß†w«æüÌá—;Ù[³r*D:£¤¤€áÆçé5	Õá/T¼Ï'$”*¸|ÿ %J­¨xœ340031QHÎÏ+)JL.)ÖKÏgğ¸·ğVâœmÓş
'Í<`uí¦¨yÙt 0L»xœEŒ±€0k<Å‹Š40#À!zB„ Ñç)bw"(è,û|ÉºÕz&‰ÒÇ]Å:Â–¢(5¨jt9¦ÖÅ­K«ï^0×0€‰iĞbË‹…Â®,s‰t¡ú‡Æ|úÜşn<]a*f¢#xœ340031QH,(ÈÉLN,ÉÌÏ‹O-*Ê/ÒKÏgH©µŞô2cÖSkæw;e&ßzÄù_Ğ¢>)1%¾(µ°4µ¸¡^GğÌv;Ójn
×nşç“Æ —UŸœŸ—´ I±½YË¡5“_ï½SÿëjŠÒå«®Á——äç"”¾TIŞ©Ø1£Er}ï®s¥Â”¦vªB•¦äç&f"9ùÚÂıÜ‹B™Kî&¬»[2wéw£M!P¥iùEI™)©HŠÿÕœ¸X%qäè¬ƒóCØY34Ù:Agæ•¤å%æÄ§•¥!ôX3ô“Jùb•?õ/ƒö?¯;ÎÛ¡zr‹Š3s2óÒÊM'{¥m\ÃŸtüĞ?Îì+™íŠPåyù%ñiù¥y)Õâk"ÌûL2åVœtk>1kuTui^biIF~QfU*’†“;²eÏ°å®
´Zr©ïÈ´‡÷Ö„Ã5`sÏİÙ‹]<ø£ôòE–İxİªzQõu°2TCPi
Z
?[üº!§,~ë±ƒ–1AÿÏs ßÆà}¶5xœ½“1OÃ0„gûW<u@6ŠÒ½ : ÇyI­&q°Büwìº	©™¢Ø÷î¾³å^È½¨ä`nïŒÑ†RÕöÚ8`”¬jåvC‘Kİ®û}½Æ °+Ê)u=B!ÊG|ĞºÃ(Xgéà“’Û™á¥ÕĞI`—É‡{{sºÄ8Z7ÁÄ LŞƒÉ!2QƒêšJHLSÉ_ÎÏR€€‘j$xfQ2¢”¨*ÎÙüÚ2ÌàbÑÒ[m’½üLJ<â¨¯Dcqb~À÷TŞ¢µáıá«®Î@êÃ±dP¢ª±ĞŠş9î¾Ä¼asµ ¸³³Ü„ØÙ?ëTs28¦Oa<;¡?G¾5¢g$ÂLVÿYÄOŸ/B¬óOÄ‹ƒûñ’·şQ<…e†ü§é(ô…¿Bi(Éî<xœ»Æö”m‚=wbAANfrbIf~ŞDOy$®kQQ~‘¦‚g±#’’>ndî%]G4š
Iùù9\µ\\i¥yÉ¨ú's2JË Û¡€¬ÀQÒÓÕ\œœE©%¥Ey
è’z¨ô3ª"»pò>F/d?N¾Í¤:ÙœÉEMS$ššj «§jmæ‚Fxœ»Æ¶ƒm‚={r~^ZNfòDM	(³Äµ¨(¿HSÁ³Ø*×«ÊcîVVsFV¦¡©”ŸŸÃUËÅ•Vš—ŒĞ4™ƒQFÅD˜Œ-£”!š]Õ\œœE©%¥Ey
(2zóúÕ`®˜¼—ÑæğÉ‚L±pñr¦($qµIÌa ½PÚ¹Lxœ­“½nƒ0Çgû)®UhÙ‘2Di‡íTb(
`d›VUÔwïùƒ€IŠT©,È¾»ÿıîÃË¬ä÷J‹æQJ!)­šNHQYé÷~Ÿæ¢YwÇrÍUE”ê¯.ˆ ¥eŸk8QRµšË–Õ;qà€J®”É`?t¬Ú’”‚ógu)9pÍªZ¹»†u¯ÎùÍÇ|SZôm«œÃİ$yö·Š½¸¥( ç©Ir³¶ªÍ‘\÷²5†é¢"ü9çÔQ‚¹.İGÀğ—xÓ-Æ~…‘Ón.„?¸Î¡ÂEÛB=ßâ©'µ(¸¢hàœ-ˆİ€‰'XÒ‚¡ŞÉN×ôš%ó^Q2VOÉB)gêP×L™;øöƒÉq=§˜v3Ü§[eB¸={ÆÁš·é¬³ƒSÁj5æ…^K`ş*VĞ•–À°üUÇA›[œm&ÔBêi†,È— 1²à½™KàşdÅü)™½ŠsÕÛß¦1ãş×¡L‡€ïa~ ô§™ôí†Zxœ»Æ6“m‚=[J~nbfŞDOQËµ¨(¿HSÁ³Ø"Ñ§ÂeíQRvA(ÑĞTHÊÏÏáªåâJ+ÍK†k˜ÌÉ(-„d”TØQÒÅ†j.NÎ¢Ô’Ò¢<$q=˜9PK¸8k'Ïb”‚ºaò>F/¨‹'ßfRlÎd“©`ŠDÈT ƒˆM'à‡dxœ»Ævœm‚=gZ~QRfJJjŞDOi8Çµ¨(¿HSÁ³Ø.İ§Â‰àìQÒtCQ«¡©”ŸŸÃUËÅ•Vš—Œ¬s2'£´ªÉ
I;FIStk«¹89‹RKJ‹òP¥ôígTE¸hò>F/„_&ßfRlÎd$_Á‰"_ ÿ\¨éˆkxœ»ÆÖÀ>Á/3¯$µ(/1'8µ¨,µh¢§†*ªˆkQQ~‘¦‚g±'ªÂ>>4‘=JÆ˜Z54’òós¸j¹¸ÒJó’1šÌÉ(-³¬CM£¤ŠM…j.NÎ¢Ô’Ò¢<=ÓûUÑÜ9y£š¯'ßfRlÎd®²‚)Se5 é3u´7xœ½“?OÃ0ÅgûSœ: Eé^‰!Xê€\÷’ZMìÈ@ñİ±ë&„&tìå|ïİïùäNÈ½¨dpŞ´÷ÖK©j;c=0Jµò»°)¥i—İ¾^bjpÊ)õŸBĞÂºh”®bpŞéá‹’»‘å7¥UĞÂõDÃáÁ½œ‡1M2²èƒÕ1OúAi¶Og“ÿİ®¹Ìt=Ç»°3'BJ‰ª²Ö•·aW3™£aŸlrZÎ"Sq{M%‡ÿ~L-:—vWkH³ÅtMlÑÕ8hE÷šOßò‡gî‡°º™OØ£Û]¥Ñ£¦U“'pœ?ŒãÅŸçé×Vt,Âd Áì²a¢ş|â||@±9ùW¾Oæ9•òß´}cıh5€æ>xœûÂvm‚=cîD'CáÜÄ¢âŒÄœÌ¼t×¢¢ü"MÏâ‰]zwéL|¡8™›QU MÂdFeÆÜÉYŒÒBhRzÅ“g3êO>ÃTğIor “ùä.¦d0¯ Q*Õç‚$xœûÂ¶ƒm‚=G^~‰[~i^ÊDoIÛµ¨(¿HSÁ³Ø&9E…Î>¢á‡¬PCS!)??‡«–‹+­4/ID:U!bH•B5gYb‘ŠU
0“ı%Ğ\ÔÀY”ZRZ”‡ªIaÏä¥Œªp÷M¾Æè÷Õd¦„Ì¦HdÕÉrÌa \!^Lå€xœÛÁö…m‚=wi^biIF~QfUêD7M$nŠkQQ~‘¦‚gqh#\M»*w(’–ÍÊÜÈ²w%P¸dd1T@Ò>Y—QÊ‹•Õ\œœE©%¥Ey˜ÒCqÏä
F5!$>DÆäB&d¯MŞÃ¤6Y–ÉÙu“£™¢ĞÔÔ  {şja¾>xœÅS±NÃ0ã¯8u¨¥{%†‚X@¢bpK°šÄÁç!ÔÇ®IJİÂ†˜¢Ü½{÷Ş³İ	¹‚ìÉêæÆmSM§Î’Y¥ìK¿É¥nİ¶Z ĞŒ¥ŒÙ÷aµ*„UºİYÓK,¹Å=¾öH6î+ûVG¸ˆ¦R¸¥ÇãOa£uí‰ÚŞ´àˆÑ“ì×FhP­ES
‰ç6'?±OšN”*aNÌFCŒ%ªs”¯ˆcó§jtõò32Yâ$øRÔ„“æ;|‹áù³t‡ Ú*©ôÑdP ª&hD÷ºÏá“½^ÂòÒ³Fñ¬n¢rÒ†ıDlÑŒ(–0ßW<(¹>\¹%¬èÛ/Ç4sˆ]väøÍüÚˆ;/ÁO’Å¸d¢ÿÇPÈº—ë~İ×[»·úàË|ÀôÛˆté}ë\Mªxœ340031QH-*Ê/Š/-ÉÌ)ÖKÏgX~ÆÕäêïğŒtï%ö—TcÅv	>X  CÓ°€xœUmkÛ0şlÿŠk`Ãfó½#FèØ ci7XWŠâÈæIqÛ”ü÷İI²ã´±A‰í{yîô<wjË²’¸RRİQé0u+•(&™l4|ÍkûĞF‰¦Ğ“0:V‰53RÁ¤f³]¥™¬g…œÊİNÎègêC„l(wÕ–ÅÌÖÔÖ!ÓíŠ«Tªb¶c-[ç¥Ğæ|-J1İ0õÄÖ‚*Õ"SrjxİVÌp‹Š-kÃãàİï}…@ºÀ3)–ı3°ÃË|6õmµ‘õÙÿÌÜÛaìŒ”•U²(8Æa8›ÁbÃ³Ò¿âZ£ˆ22iÈ±İòLä"ƒzpbçL4Dfã%óm“€ŠĞë"’Bš¦NşVØ<‡•ºBà|Š5ø2ä`L rğc“.|Qo¸V¢^¶,ã‹¼”\Q©í)Šã8¿'øšm«Š›­Â³ª-ÇÏ}HŞ–³Jópoyü&¥YÜƒkÔ•ƒ‹Ğ ĞŒEÑÆ:P`¾›¿QÃ£¸Šw¼r"Í…èx3&û¸Şè¸‡~¦ÕRsi”7;®é`Ú$ Kâš(Š†:= «ØQ¡–o¸Ëé²ET“G“7ïº_Í„%#ç!%aşY¬g%½íŸ´O0wİëô¦yP¬¥v|aëC#*—ß§[%)ú¶HËÏ÷áÎ°êbÍºrãĞóÕ¬½u¬Ü¹GÒ»<wÃfÂpI¹YÒ¡ïŒ:ş`Ç<·C¸›œ5GiQBd¸f%]Hj”	t‡™?µÇõ(·årÔõ£ì™ó‡÷1	‘dç%áªaÕ’«+wµœ{Šé6L?û€‹W–QÊ@.Â&0º¦Ò/üáDü/\'ê§Wº8íXÈ5·­Ç$yj9xµ>’¤fûÑ·^ªÓòõÈë5÷iC3-
™GğÿìµÏòq…øğp™kÚFxZX¹5¾aõ]WNçœ«±fÃò­x&ñ˜Qü¿ÏF3X–Üöõ»[ÇÑ¦îªN²6]n†Hn hß:\6Ÿä›²ïûğE¢›˜Ñ5˜Ànlãaí\Ùë7úí;ê›6ş+-|ÕD<î—pşˆ`­E©xœ340031QH.-.ÉÏO/*HO-*Ê/ÒKÏgøò4‚u¯Ü¤#\[ÛU-™'š¿ÀµÎ¢U¡à÷]§·IÔ•0=ºåDLõ$æß
ã‹ŠSÁê•/nTNş–^À½ÙtåäıÇŞl`ÿf	 +£7–¾¹xœÍ–=oÛ0†gëW
°¥=›ÇA†EÓt):0â…>˜_%OIÓ"ÿ½¤h'ĞEƒ@k‘päİûŞ#ê Ç»—À¤wİ•÷ÖWjg=±E5›j˜WñAZ+ÔÒ*ndm½lRBÓY!m€”ziM nˆÍ%Ò¾¿¯;«\ï¹æi×;o×Ú)NĞ¸ƒŒUr^3T™WËªzèMÇvğtU¾q…‚Z3\$Q††VLCÉ| F®˜ â¨ÓÜ}Ï±ù¶d×¹?ö§šy ŞöáØrŒÌ¾")¸`éz×I×ßä·±8ˆaï*&]F9‡%K)ô)Ècää-E7ÙVZ9:LÑ[âÔÁ!=Ô7æ1Iµ^öb{ƒ¯X†¸v,½ŠzgŸË¸ğR½¼g?(ì¨¡“øddZå‹ç«_(Œâò‘‹/ğ³›‹y“?ËS³³´µ½EØœÄ'#s…äÎ´=í­ÇßPËá¯&C“EâqÁ.áq„¶Öß£Pf¿ªOÆæ3x!Äy¿ƒ#áÜo¸ºÿ¾¡ÿX˜pød­QŒ6Vs,sz²ôYÎâÖ9•¾ÆR¿9ÿèŸ×qi‚ÅAü—˜Ç½ÉxœVßkÛ0~ÿ
‘‡a—Tft–‘‡5[ËVF7ØÃ­k+®—Ø2’Ü.”şï»ÓÛrœ†–øî>ùîût'ÕIºNrFrQ§_„à"Š²æB‘0˜LUQ²iLz.›™æ…zlhÊË8+ÖÅùc"¶IVÄµàŠ?4«sÅÊz“(Kp±,ÖĞøéí–ëëun\R;8Ï7Œæ|“T9å"1Ÿ8å;ä—*QDA ¶u[‘J4©"/Áä‡ ğèµèşûÜÿ•¼šOÍ"3^˜ºÚNïƒIVTŠø…áz>èg¡6ß/ª|¤0ÄG}“.p/ª”¹ùÌTRl°¬2©Ü·Í­‰fY2(¹¬	ŠKñu7Kâ!_-É×–d`†‰U’2äùš)Cuõ˜ş[»¡¹çŒÜJÀÂ¢¤_1-~¤¦BM½:Ú˜4¹ÖìC€c€`u4Ø’€r5Öù3+ÂNÔ0]K&,Ø2n³v´wÎ^wd/ux¿äÙ,Z_P“cì-“5¯$wë¹VM•’ö|íz{W„q$ÏHŸ¿Ü$÷~¼\Ü®çòÆş«mÃ¹ÙåºİÀˆßŸ›§›	LZHc3­6ĞÊF¹œĞjU@ÍÌà-¥sCç#p ÁT#*7#Z~B¨ñÌ#2 Kr8¶ÒÓÈCPÈ}j¼a|´	Ø³À)¥që…å>8^“×~Rº¯÷¦4Ş·6]áB'zZ~«ûyhßşDŒ›×³ÑNË§?GNÚM¯†¹à!°pÛû´\Œ)?5¸?= kğvAHØ NÕrd%µ!‡d=23[qİ)·ĞA£9~•İd„v7-‘Î7¸ÌS"ÚE;‘í*æêB?IÎÚYáÚqLvÆ2IƒK‘$	Éo¿/ít6€(N$«2ı9Âª¼¨Ø{§=¦+N]Óâ¦~ØvşÉµc*
& pÍ27¥İÕ.©x©Ã^üQìše0´¡Ãß¿m_GƒQm6õ`Rw›g0¬ûŠÑ+.ÊDiéíÕòâââƒ™áPâ/¸Aº%È)˜öŒa[\Ôjå¡¨f­İßDşƒ›L•l¬Èı½ĞÛU¿K^=1¡0>´îÜ‰Q®©Øë´ëRä´X8gùCL0ÏÆˆ>bB&İeÌÔ«iÑi{ÁñİQ!ĞVß?†153~¦=¾°¡êTõån+xz+'·'¸êë½s<›²ĞÚâßk«Ycå5øå¹A½±xœÍ–Qo›0ÇŸÃ§ğò¤Ş+å!KSµÒRMk¶=V®¹€°™}¬İ¦~÷†’ªnlêKøçßmşä\ìx,AÌWÆhãy2ËµA6ñFc–#cnb­ã‚X§\Å6q›\„BG`K”áK­,r…lKLŠ»@è,ŒäNÎn~ğH†±eR=CÈò”#„ù.¦,U\è²Œël¤ıËLÕïmJ‰(©(,êÌÕùÚ´U¶*Ámé{Ş¶P‚}äÆ‚Ë:¡æF}vYµ”ıjÍËÎæ¬,ì²¹+ƒ}o$·„ÍçLÉ´Ì1’
Á(Ş€ù¦*‚Ò=7*¸ª«£'­JT2ÎéÚnGp÷ê¯†ç%Ö”uÌ¬mÜ=°¤=1-Á©œVñ/×>2€…QŒh.Ÿvä¤ÜÁr,lâ´IêæøS¶·éˆÛHL¡\Mæ·r™Ú‰Oh[š²yïšE±÷Eâş
na¯À+û…§2â(µjŠ<+;¿_æ¡ì¸ æÉ¬¥{’³ä=>Á·,ö‚Ê†¹Öx¡õbì‹†‡èØ§Pº¤Ã]hs'£ú·ÉjxŒÏjQ`¢ü	ıt,æ\g\ö7¤-`‘ç©/ŸŞ#İğ(ô*ÛÒı‡w_ô/vÇš|.¡w•ŠßÀ™yS0‡NÖò§^æ¾¯œb•üjæ0djàÊÔé[	áIË#ê$¬@‘ã§Î* Ã“6ì|Ö–îZ[ËFf ¬ıÚÎK~-vÃ8è¶¼Hñ?¬êˆìİ9|=Ù»÷èısæéƒjxœ›+6ATº 19;1=U!½¨ Ùµ¨(¿ˆ‹+3· ¿¨DAc¢†ÆÄy{YÜ2›ã­%Üaj4’óSR‹õ<óJR‹òs&÷qšOæf.+ÜÊÜ
ağ±4sƒ~ù%nù¥y“Ù"'eQ‡H>äH›ÜÆªá¬amƒ0ÌØš!Œ½lMü †cAANfrbIf~Şä÷‘“—³+@œfoE´‹CÆiÅçÊÙUº\m0å“Ù¹Å _W©xœ340031QH.-.ÉÏÏ())ˆO-*Ê/ÒKÏgxõì¹Ç›gk¯ï¹"x¤uÓÉëÇ!êQ>áúS^lÖgüPÆÇéu}'Cû…ñ‰EÅ©`õ™ÂFZN¢Qß3§™Æ,²‹Ù w¹7E¾»xœÍ–;oÂ0…gü+¬H@önŠÚ¡}-Uß†+âG›VmÕÿ^;!@%–‘É’ä\_ŸãO–+²­ÈoˆìµsÆ1†ÊG|È‰JC%ñÏ„
ÆFÍ.IhâI´©ÖÓÌ¨Tâ'á¾„Ä47…™3eAÚmf»¾´%a#ÆŞ*ñ|ŞxŸgQ „F×Y†™‘ÀQÓ˜+(Ë³$‡:s	$°(¹ö¥Ñ^›Ûˆß4Ká?là€*§ùÅnu^<"pÉÃõo%S_?Ø/ıä ë±cß4÷9š"é®ÌwJ›-¨‹&V¨ìõUµXs6ïWBŞÃ{%ë\~Ê^ò@zº2ŸÃ‘/ü²ßÿŒ|à·3ŠB¨5ï—LëÒ‰Ëg2û³Ü5+CKSi…MkŞ/™Ö¥—'=«hc~C6OZìôËçØ©£¥qk”âÊ{÷~éìm:¡¹ÕN‹âÜ¸(|NDè—Ô	ÃNÌF	Œ³—ë³< gÖúO^¼Ÿ#ÿóŞ>3‹‘ áÙù/Ù0¼·xœ•V]o›0}†_añĞÁÆÈ£1ik«¶“ÚUk¤=TUë'aŒŒ³,šößw¯?&!ÉòĞÂõ=æø\ßc×4{¥F–RÖ—BpáûyYs!Iè{«2>Ë«ÅègÃ« 9Ç¿“#à³ÌKøğ°Èår5M2^ê×ÅˆádMàG¾/7uûÒH±Ê$ùã{’ÊUCà—W’¸¿üà8hTJÌË\²²–›àÅ÷ÎùŒé¤Af.h’ËB£àû° = ‰).ê¶±‰ƒ¨²Y¸˜&i^à²JZ?jÜ“Œ’FçõY––\ÖUMğu—¥IqÈ×FdP†‰9Íê|Å¤–:Œp „oZ_ŒF«òQãN¶zEQw3•°ªW¨²uHÉiÂ.Tu ÷&9°#§ÂÚñ™‘}'«O×È¶ÖvãÛÁ^õG‡:¼á³DTE}ï‡€’Lx¸V»=ùÎššWSa‘Ô‹unä{£™ğ+Qg6¦k§‚‚ÎWUFîØúÚvf§Z1±ÕˆIWè˜À"ºÜ¾×·ˆmÎqJÎÌ3DM‡u¨N„ n†±Ş“ªÏ ¤*®cº‹ E5Y–FM¹pÄ0Óx£ıXë~Ç×a ‚'˜\‰ÊÚG+Lk|k‚éU—dqlÒ)¦ƒH€û ª×K.L 6ª€¤Fmÿø*œ^uÙ`|˜Ëş–6<”Å¦ª¼§qp]Àå¡Æ†‰pÃF{wª·Öi|ºsji­9Ôç‚'Bj7ôi\8˜KÍ$Ós½îXK·a8µ–{,²_R“r¨¬Gì´-®=òR•´—ãM³5Ahpc¡dÊyÓü¢¢½^X³n§Ñ÷äsƒÈ¸µ·'ÏÀvÕEçœÃYÉ	œ¡_¾İ—€Öu‘gTæ¼Õ‚OV¾Ó· ¸Ñ€w¯'küß	(è½Î%’Z‘ëÉäXƒ×±O±ÿ89pÅÆBÀ®AÂ'ù^>7æAÒ”|À\›œê©uê^*Z<0ñ‹	}ñSv»N®1pÆªF™÷(M“PIƒÒßŠ¶ò›aXm‚ê…Q4°gô(y|šn¤ÚnÓ˜<ã
q ¹¥¢YÒ"<[²ÎäSœjqÁ«7’Ì¹X0‰Âgµ±$S8È˜|œ±9ê-9/x^õÉ°¸‡™Ùåo-‡Ù=¡‚åê@g*_ôö-«Ø:lw“÷LU8²/Z§&Jô[è0W›û©ÃĞ¾xœuÍ
Â0„ÏîS,(æàˆ =yk»¦E›„MD‹øî¶V­?x
Ìff¾ñ”íÉ0ú½(+ï$bƒ¡q²fäÄè³Î¤öÑéíı‚ĞW,å®^R''9f®ò$Ğ?²9Æ‚± PpşÒaw´Ù—;yÍß´QJk²ˆ¼À@8ÅbG2šv­ibóWØz³­#dª¿T¥àzßÑûÂ›ô¨èQÚú»!ùA}´§³ú…³eiJfâªÈî¸å»xã”ì«xœ340075UH-*Ê/ÒKÏg¸´oÃNşÍS-ÎuÕ«–•eŠª4„¨)J-.ÈÏ+N)»ï`·úÉ¯…¯cÓ-ïì˜‘Ğ¦5ùûT Iö  ´GxœuSMkÜ0=[¿bbh‘R£ö¼°¹„rH/Û[EñÊ»JlÙHò†Ôø¿wô±¶²°àõxŞ{£÷Fƒ¨_ÄA‚‘vèµ•„¨nèJŠRêºß+}øşl{]b¡é\I!îmğÓ˜Û£P¬3cí`"Å½´Ö³aa¤¸£•  é)~)Ùî-@'†‡Øò¨´“¦µœfRüñ´KûLH3ê(¾.bÌÿëeIÃ«>Õı?Øl¡Ä›(‘ŞTã¹xãjZµ[Àãğİ€4–ğ$ëĞ6•Õ
âI­‚r."‘±•ÚÄãiÜÀ@zÖÎy$~UîéèÓ—o§9	$<"ñèø3ÒFçCù®di~æ
š:›ã-ÏX|øq,ÊÖÇ D}\Ìka×Ü6¨œ4C×b1÷¤ùHhÛ"èú]0vyŒñ‘‡rªà¯Á¯¿ÆEK;¶pÆNzb—rŞI¢/ò-uWpíx^/¶nà´D|Ş®YçÕ›7Å ÖÎÔ|Äö G2—ü°aùï4~Sû°ÙèŞ_€åhJÂuùl“Š’<±,ğßò5ÓÎŞÑf4_óú”7€Ğùo“W ÌÁç<óé²ÚÇ	ó€¤‘É˜û?èzEµ‘xœZ[sÛº~¶ª™´RëH¹™zÆIçÒÚÇrN2™˜„$Ô$Á‚5ãÿŞ]\H€))y‰|»À.ö
¢¤É]2"YUŠ¢b‡‡</…Td|x0bE"R^,gÿ©D1Â)…¬ğ×"Wøß’«Õúvšˆ|–ò;ştEå†¦|¶OsHñT±¼Ì¨b³òn9K`EM™‰%şW05[)UâïJI@Ü'‡‡÷TâÎ¤|KÓköß5«‰ü;!fOÓKö0$ÑàÑDS¿ò–§)+®Y%Ö2aÔ5µ¡Á–É¥PïÅºHch3,Ñ`K{%Ù©(R®¸(ŞS±´Ÿ°‰Ã’…[.Ÿ
ÅdA³9“÷L!M/‡%•›9Ëç†çL¬U›A”Åô_
ºV+!ùÿXD!½µä ‰EÆ“èI¶ÉÖ’^0à•‚vßd™xh¯’,)à(¨A×j¼§O?H0Â›MÉ†˜X,Y"˜(@‡LN3Î
õéİg9g‰djI¢ÁäÓ;z¯4Şr{cuDñĞOEÊc¹X¾É´ŒÏÜ:ìÎ~”\:umeÇ:”ôš¥0˜¨/×Ÿ¶«KZ0tÈfˆ¶ÆûÙTé¿ThÖ9û,¯hU=S‘£_[0ê»´pËîš-À¿W7â]ÏÙY,Qî(Ûçèy+§PÏ6¾1£¢SZÀBoÙÉ pÊ‹5*"‰F“[†ªñÎÓq^U¿·)]cInÀ>ıGn5ºÃ¡ÙV@ä¬1IXUõhºmÛ£iQWÑCŒBEŸíç­øc\¶½“ÀûÏßêıß»ğqCq›;m¢‡B´³‘±şÌÔ²À†¢\ğÂ¹Ñ9+–jÕeğ›I`u#Ä|…•A•ÀTç“äëƒQí›ùºÒfIÉ…ßO@PĞŒ´"™ S;ŠÎF`Ğ&ğááÔÛ‘Iúç‚È1ÀÇ#íøaüsÎÔY^ª‹JÃhœ´[aHT‡§Q#©ÛÚ¥æ½›¤ngÄr÷Ø!÷¸mõ²ÓbøfµĞÉ):º¢,®8ÚÍ-aafNê€å–C©¥6©9Ê×¸Ì)¸>üÇiVí°gë¦IC4´”YRT%¤ºXy_×4-sˆXcä\Q©bµU‡'’ nõÑ*`¥³®²±x^j]éc“<™¯Mhx^<Ã#!´ŒnCã_´ñı5:à_Eøg«ÒÍ5[rHe~4wøç¾¿ìÔøçmşM%İÏË6~(ŒşUß_ÇjüßÛx«¿¾·½ âŸwôùùæêœç,‹&«VQø×¯Ûû?çÅ4?xÇşe€w-B· üëîy™ÍÎY–½¥*YuğÏ£x"¦Ÿ×]ıc[Ã<æ{ ¤·Æ÷·1ß9¯şæAŸ×k×Y¬‹„|`†1ÁãŒ×MÀÉ$ÖR?pğ„,5ÛDõAõÀQ;8¨„VŒì1¤;¨ Ö² 1ßiH:]k”²F5„Î£x7ÙÀ»Íi”°×|N‘5Ê*°¼†Ü?Ğ(hÈü¸%ó™sß(‰›làmë‰’µA¾jÂ¦óˆ´ÓTÓDús^Ûåë§¡ÎH´KxŸË`N¹ÇRtW†Të;sgèm’5ûXC×(Àö@z Òìè)}}ugáø@_×Ş%ÜY'a/Ó=Ï üBÚlÌkvT@Ğ¯Ô\ë*sGWj7šQ¼ ®×c_Pv7½<Šp¶@:<HÙ‚®³¸¶BØãác“8>ŞÜ\äX×¼à|~â°	BO¹·ŸSCóù_Ş‚£ÊlfÔÅjòE%f(vµœ¦v,¿…µuø“ßƒ´á“PrR 2.„ÌµcDù(0	è@F( ç1!’8>@w50„Ÿeifh/à°y™±Ó•€:¤ò‰s;Í«‹Q‹{–^1PBì³M@s¤l&#ôm?™îµœ3öY­˜Ì„1"p0ª|u!R¾àáñcËÛñ¸Õ•?1 a%%Fğ7?P¹qË'Tn²¾ŒSOŒC­»!Ñ"ktë}ˆÈ¯Fkÿâ:fg›œ™×
Ø`hzJ¯©¯bgm‹¶ğ¼í`ü;öÑÜpDŒ±¿Zåİ‹ñè‚&f`Ş^•Ö31Í `ôˆê'uèÀ2!1Š²Çi¿0˜VS™©X€ìx£Ä}OèÂ?ˆ"t	G`æz*&[fnĞ¤éÖÎ¡nº~z5rêSÈ çT.YL1L#ˆ‚dˆéçµ%2Å2Æ/ü5¼«‹ùOµ.ñc!K/À3)·¡Õó$G€ùŠÒ»–^Ób‰µÇ¬£Zğ¶Êúî["Ğ\}4Ğg(ª ^Ğ¸ÖY3Ù¯ôFKÒ§¿äDÑã1ÿã•‰R,‘òfv02·`òG±Ìy‡Šõ¦íiÇìU$w-;Õ#±¨¤ÅÇJV@J‚ÕŒjHZOÆ6\.%MYÌ7ÖfjWçˆ‡Ï=ø€)_ĞbcÕdq4cÈ"§ôX·tlW¾ç,K«!/[iYhà ³y÷şÏÙ’f×ŒV¢¨Zi§¾ı€„@2ÄÁ~50Âu¸åñègàh¸ÿ”C}ƒ‰«[&ğf*u?@Ùù@7í¬»4ÃÑ¦}×V3z–xÊˆlÃ=’ìº¹Kôß™Ä»SŒ4.BùLBîÆ‡ŠğûJıî›¬—l) ĞTayo q+AŠ=Ñj½XğD·o
ª¦Ğğ¸7M…º½(ß1¥cLèü¢/6qC8û¡ĞÇ;VÀìxŒŠ)h¿îŞy=æÄ…T İ¦|y¡§;–¾ Ó½”é°&Øné6‹Ÿè&ëë³ã—ßøùCöÉô|\Mİ29cn"ÿœ¾¼¶Oa¾N4ÃwTQwc¨ıjAöóÑ¿xüßÈR@‰œ+ıgôÇáÁÆé¥¹Ğ´`ûŸ¥Í2$G!n˜FŠ¦SÕ›íÀŠJ#¿‡„»,e‘F‡¹ÓèøÊæßß²ø“Küá3ëëáµ4ü¼²Õâdnî\4øœ–_‹oşñû«}¼¹8¿²ç|+DÖ¯u+K¦nÆ‡-Jjd°7ç—ìÁ7Øñ„ü50àŸµ…ÿÙÿ‰¶qÜ{ûqäÍ¾ÖG1q$Ç­;ànP Óãeş|lü{,CA&dÎúàİË×WLâ©öÖ‚ØZr˜¹İ¯±ë:œDx[ÁX‚?våßUŞÚeMàÚoßš{Õ¸½¾uêİ¼]¦ÁïºšxÔÆ¦µñŸˆ™;34N06¾õX­ËœƒßuÇyµR¾Ó'€»ò¶!flYd‚ªß^EpáÈ¼˜˜—°µL_,`ÉÑ±L&áâ³ÙÀòb9¾c+Ö¹‡joP‹³ÙÁ÷#¨X3vuÍŠr]`µ4=¥YÆäøùa™XšWY1Æß¦ÖL¦9öc•1Ş#3¡<œÅÙ9óş÷D¯¸@¸5òädóa‡ğsÔìw¢y<NêL6xs…/Í`
”,–_A#ß@Ë°¦sºœÍÜQ†îÜç=“€~ú_ñ¦c,
&şÇ¼Óåúê†¨@ts“)€/ˆÃüé/‘Íıvhg`à·K68-p³ë#hD§Óéö˜Òş¦	F6›Á©Y#2ÉTm¦Ğ×\ªm›ëßîkXü3(P;JÇıik£-#Æ¤“üdcö>	‚n˜]|büü
ùli—ô‰´÷×¼Ÿ‡Ó£^!cÅ˜NÈ?È³(5ıúì›¢Cç	Ò¼éBê×o·¥OìÖ:%æşé•ÕŠfcÙÄ„ÛAû(Òñƒ‘Ùş[BÙ µ	kõÔ‘7ÿ05½îx‚† ŸÜâıøS<L8å‘‚º¶Ry†¯T ­™Y’Pù„eÕn\iYf¶ú7Ë·2×–æ”EÏ¾KØ„ıSkÈz\àmº®Ğt1Z`\:òíoâüOÇYŒ#¶Eµïd°Çë)•õm¡%'®N°Ÿ‹zÎ£Í/;ë¬oŞÆ{’[c®Ú
ÓUtg6@d–f‡ö‹ÒVI,..IàÊmyì{	l‰öØ#ìØ}Û*±î/r½Ä^2;ªı…ö){¤®?Æm»Fî/w³È^‚×dûKöˆ®]şKqWˆ‡b«ô>xKí¥Ÿr5´©{4á½ iîa¶ª$Jµ¿nâ‹ï¥¤(‹ıµÕË¦GmÁS¢mÚòÁû+)|µ´n|ÊıUÒ¦îÑDóîl›jäş:ğ·í£€šléÒÑ½%Ûdo ¿A`?ù/„$ûÅØáH{³‡}´=yà¯ä÷îh¿Ôa¨~%s4”Ôÿ_”ùøí
™xœ›Zt§xÃgvÏ¼²ÄœÌ”Éï9Ä'à›<…C£ ±¸¸<¿(EI“‹Óµ¨ª" *êœ˜——_œ˜›:ù £èäxv6	˜…d°œB1P¤yòA!ñÉì¬ÿ8B‹S‹ò€‚›¹Ø.që¢¨£€Ï’Í×íÔ(à@›ºRxœ”=oƒ0@çøWœ˜ ª CÕ!jÖJ]ÚJ­ºb¨c;ş¨T!ş{mL"J²`|~wºç“,q±Ã¹«¢µÊ@ŒÑœcM2½g‘ÛZ“¹ŸŠš/›§…¨3FóLî#” ”eğÌÍ«z±Œ"Æ*®A*!‰b?P^ÒÊ*²W.õ£îQiyqLŒ9PNN)hĞ"”<7İwü>1£Û5eI‹Ú®7¡©¡ßä†¶ 
0È¾p[çD…fGe'š¦%pxÜÀÊo&V
%fÚ9,Ú[œŸ˜À×ØvüaƒdçT†£äœ½0Œş¤é×ÿrc“s†2_ar4#¡U:g@3ïáİ(Ê«+. $Óar¬İ’3¸WôÀfQtÉ2MXÖš¡x–ãê_TüpÅ¤`o(÷©G=ô‰±¥s: ñ¤úMïf¼§l¨6 ÿq]J4í„Ş8oiFb¿²”Ã¦°?xœ}SM‹Û0=[¿âÕ'L÷^È­¥ìeYØŞJ)Š=NÄÚ’‘&Û†’ÿŞ‘d'«îRÈÁiŞ×Lİ?ëay>(eæÅyF£ª:°7öjÕ*uw‡§t¼·O“é	&@c<Ù³f6“ö`‡ú7ŒÅ¹Æã™rÕ;+@§U|]¢4™¤Ãdãû|l±wnÂUÎãg‡=>íàµ•é¡ÜTf”òn'2â©òÄ'o!T$Ç‹Š¿µ6ê)ºd‹îé3M"˜iÈjõ¤™<rGH0>Fã…2+‹b=–ƒ	zı&±9¬o šGÖq´ÏZÑöŒpÚßP^]Ò#{‹Óü:’d»L†±—ö(½]cü¯&IîJ¸[¨Âşe^ø¼ñˆ!K/âşÛUÌ6»–|ë:\ä+YV)Ì(í­ğM¬ªrıQ{q„ëJ}LM¾ìP£n3à½°‹¸¨&A`ØLbI(î¡lk’Š‘ì¶.%ãšK$L¦"eÆ»Î&¥!³‰lYá®C±Vi?”k}Õ± Ğ‹ºu_®i~ûÇV±hbpv„;^Ê¿î…Rõ;«sÕ˜ÇõúpQì	S–<xœ+HLÎNLOU(ÈNç È9£xœ343 …œÄ¼”Ì¼t†ÖŠÓ«Kı5³*uZz?¹Ø8 Ø¬ß¢xœ31 …äü¼´ÌôÒ¢Ä’ü"†WK~%:‘¡]ôQrÖ¾Å3*sMÀÊRRs2ËR‹*—¾ŸY³o}üİmM“lczV5ª$?713¡¶åÆÖi÷³âœ/XêØè½[1q'C$TAI>Ã$–ÉkŸ>ùôFm¿nâ¦OÖ*k³ï‚È¦V$§”dæç1(W%ùıåªó­v0oê°¨ÉÊObxäøü`ÙÜtßKúSÊ]˜e%?q¾€È¥ägıQÉ`[dºù›©“ğüSâë5Ôæµı?wD	¢¨$µ¸¤˜ÁÖ>ÙşË¤µ÷š®êoû&çç5Í^
"_ZœšœXœÊpÌ˜ŸëÇ3S·X·r^UÎ>0ÿ> |€®xœ31 …ô¢‚d†K÷~ªßÕ	ÜĞbë“MûæN]`–Î())`ØVö}Û…5ì;Or­{ì{ª¾ßvD:;1-;‘aÁD‰ãpñJŸÛ
%^ï5\ÙSs `ÿ(¥£xœ340031QH,-ÉˆO/*HOÎÏ+)ÊÏÉI-ÒKÏgğ÷9\^(~HOÏãÃÜEW?Ÿ›d³ ÓG®ªxœ340031QH,-ÉˆOÉÏMÌÌÓKÏgPÕš©2õqât']…•¿fÅ§±?kb 
¹ù)©9¡¯,e6e|?È“öìÇ»eùÇŞ¾³ ö¥F¢xœ340031QH,(ĞKÏgàn<¤#¿2=Ì[ÈHÊ«ìæ£Òƒ- ¦šŠíƒÙoxœ›Ä˜9Acb ®¯Ï¢âàº»İ¥¹¼ÓÿîñdŸ»ç ¡n*é+xœËÌœ*ò€Ï@.éÙºMï9'Ep©ºUõ	 Ù¢xœ31 …äü¼´ÌôÒ¢Ä’ü"†•{+oå8~ü5]ïî¨Ü+AÉ[=LÀÊRRs2ËR‹*æyüê¨¼±Ú#Ÿo{»»Ş¾gÏ:¡Jòs3ó®soœÕüÏMLxåYA1† î+|ÖP%ùÇ^Æ™>vúM[j×)­ÔU“·i_…È¦V$§”dæç1¬áıšeôÒÇÈ·÷Iÿ±R#OŞ/P²ò“æš—ÍÜüúÒá—ß0vÏÓ/„È¥ägıQÉ°HÍK¥ø»ÃrİÜdî€93&¬çpà†(*I-.)fø.òtíÁjë¢¢UÆ•?dÔ&†ÿí‘/-NMN,Neˆò?ç29Ü+¡êÆ²Â—²»yO Ãğ})¡xœ340031QÈO,-ÉˆOÎÏKËL/-J,É/ÒKÏgÈĞ{s*dOÿã#×ŞoZ²›ç~¶& ÈFêƒÙxœ›$xU€³ 19;1=U!"‡:{bQIfrNêäLª%©¹9‰%©“™„y úe†J\u³ó'ÎpÏO,-ÉĞOIÍÉ,K-ªÔO/*HVââÌŸøC*£¤¤ $59…ÑMM.;1-;Q¿ (?¥49µ¤
$ï’Ÿ›˜™7Y€ÑT ª, 6D‹ÉY"X”Z_œY’_T	Ó7ÑŞxò&N^ˆ|iqjrbqêfWæ`FÆüÍ3X¸ÔäÅ¬ö@ò«”bK’Õl:†ê¥¦g—¤9Bã0Ûa ’ìb“ç°‹‚YQömIå€wxœ»*°Œ‚Áä¿,âç„O®`çÛÏx…I_².›ÚœÍöŠ ¸b®xœ31 …ô¢‚d†{İ¯nìJÔu}ò ñfF’Í×û&`éŒ’’††àc!ïìë½§õ¯É;¿)ÌVÂ’ãD:;1-;‘áÓ§ÅÎ§w%ôİ=»ÉAtöåµ 6®(:¤xœ340031QÈO,-ÉˆO/*HOÎÏ+)ÊÏÉI-ÒKÏgX•“i½ãş‡mKæ-šùNêøæp ı.áƒÓjxœ›Àıƒ‹· 19;1=U!?±´$c"ÿ^n0Ã%?713obƒÔäiŒÒâ`!ı°˜'DEI>TZ
*]’“s­HN-(ÉÌ‡À
¼ƒÑÊúÀ¨!…d^hqªsbqª&Ä“Í˜®B
1Ï„°6ob¾Ç i@¸æ~xœûÁõ‡sƒãæmŒŒ!úú‰.%ù
V¶
jù‰¥%@^5§¾>§_bnª•¥êtÀR.©ÅÉE™%™ùyV`) XªDLögÖÕ×ÏLSH-*RP´UÈËÌQ :¹ƒÙ–ª
Ì?Élb…U%gQjIiQH@$	7„¡rj“2Iräç§BŒ÷c‘•‚;=±¨$39'áîÉ,²"@ct@¦r!Ù0y>ËM ;GN_¤xœ340031QÈO,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgøm¼’õÖ4å(~µ_3-;góüSŸoˆ¡§(¿´¢şæÆ…»5ÊèÎm¿´hºÏ‘Å‘³˜v(–ï·ƒÈxœİ–MlAÇC¤àÒ. õ£Q£k­f·âãG›¶BQ²j¡a›{1ã2ĞMqw=4\¼˜^ ãÍÄ£gõæÑ›‰ñªñêÍÄ›³l[ ¥@bêöïıßãıŞÌÎWé(‹•÷£~ÕĞsZ~¸òf¡r9M=¹²v7—ó0²LC·Ğp¥,ĞWo§Ø‚0>)l¬ƒ*ŞRÒ©Ìº-Joıûâ¾X°õŸf@AäÆ7Ã˜)ëaAË&1ÔÉüSIÛ,²,˜G5Gºâ=6êü¤gE•<µDà.ÖÂR€ã0"6Ö]+Ğ/ŞdiÇ”q˜Í G6²HC2„1p¥Î3¸•@ ŞffÎÖUA¤÷ÂAúºoìjÛ^1˜ †ã¤£¢h@›,1›¦|G®ÊL	$¹©¡BÖr
“¨í‹ŸhİTšö–»öWi™›oh*“ÙEø†z©â‹Ğ3üıì¿]Ú.ÖˆÍÉ
Ä¶[U™¤XO@E¨V§-œm%[Wz:<DÍıÒıö(›’sĞ²Šv§¼ÒîZ©5ÒfJİj›ZĞN_r—2Í(ÿåğ8ô'«{‰ø`ÿØ•?$Nƒ|Ë½û½¿íŞñ]Ş»=¢®¶6Vmù¼±Ì&h-tr±:K$Ê¼™›óå¾[@ZhÆ}Œ™&pÓN×€ûY'tGË!¢=DN¸Ód ¨†‰h%2´—¦è#¾ıë’şâAhë<Nş'C²8A9¦¼äÈu>¶‚=!;µÚÉNu@u5İF5r`¢Í/ë–‰Ôğ$Ê®pÜÅİí&Ñ6«§‡"/şÍN­ÕĞNyËfıywº	‘™jãäD+He!ôÓà3Zâı3Îá&tĞpMÜÀà^¤¾ùæã„+xœ;¦wZoƒ£Öf=fF­É>›q+03ª™Zü\0Ñ¡L\Z™y%EùÅ©É“¸6Ï]Î ˜óæ%Gxœ;­·Ñ}C/#wz~~zNª~iifÊæ:Æ³¢“%øÅÙÒ3‹KR‹&Ûñ‡°„XE!Œ¿Br†§ÙäháHÆøÉuİÊE©:
©EE
V¶
Éz¥Å©Î‰Å©zAP©É…"Ê“÷³÷qÀÔNNd‘Ÿ'¤Äçœ‘˜—X\\_”²TU((µ°4µ¸DSÏ=µÄ-35'¥MĞU¬`¡É¦œögMv‘E7½PD†U!Ñ¥$ä–É‘¢Ş|nùEéù%(Ö£
aZ¦h=Dh²§šıhj1í-HI,Ij^bn*Ø~T!LûÑ´ íç@èG%P´Â#ÄÑH,*NÕ ¹@/4ÔÓEsòAö9äHC³ PDMÊ7?%5Gd„¦D;Lz?{ûäTi`dò òÖ…«xœ31 …Üü”Ô†%üuŒºâÅ®ËûÄ¬/\¡ĞûªÛĞÀÀÌÄD!?±´$#>%?713O/=ŸÁnÛd—]‰Â»ÌÍüÄ+4º= Ì‚Nâ†ëxœR ­ÿµµ°pÉÁ@0EĞ Œk;]ãI¸e9< “„¾8aWe[ãûÌĞÏ†ZÈ.A°Í“V-t_üéñ0 ÔÜÒ‡ØçêÛíQÙ“—9&&¢x31 …äü¼´ÌôÒ¢Ä’ü"†•{+oå8~ü5]ïî¨Ü+AÉ[=LÀÊRRs2ËR‹*¾<ˆŞŞÏÃp£¢µR§Ç§$KèÏ¨’üÜÄÌ<†FuU©v·Üâ	ŸÖd1¶Zœ´@ª $ŸÁõÆ„úŒÛä#Â•dMNËIØf~b€È¦V$§”dæç1¬áıšeôÒÇÈ·÷Iÿ±R#OŞ/Ö5YùIsÍËfn~}éğ†Ë‚o»çéBd‹Rò‹3ş¨d8¼ø°†÷Û×|B1ÒïßG-»ûÒ¢¨$µ¸¤˜á»ÈÓµ«­‹ŠVWşQ›zü·D¾´859±8•á]j›Òc7µ­•'¥¼>™ı8Àz˜ ÕÖ}"ê!xœJ µÿ²²u'»ÚoW…Ö†š“0l°²h-ğ‘‰S±)‰@"Ôäáÿ£ÕôƒHpô X'‘ï/Ü
i½½"F¤i§û˜ÆGé÷$®xœ31 …ô¢‚d†{İ¯nìJÔu}ò ñfF’Í×û&`éŒ’’†súÊ?÷~^ôÍá¶è$óg_!ÒÙ‰iÙ‰Ÿ>-¶p>½+¡ïîÁØM¢k´Ÿ(¯ ˆ)û¤xœ340031QÈO,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgXµ|QXlĞ‘i^ç×M¼>³_¼°æ¨!†¢üÒˆú£Uïm}b_qo*C•É“c_ïşy¢ ñ¸,Ÿ«xœ31 …Üü”Ô†%üuŒºâÅ®ËûÄ¬/\¡ĞûªÛĞÀÀÌÄD!?±´$#>%?713O/=Ÿ!zªØÔuOCÜ.ˆæÔ»7sKÖFUe ìßÅ¥x340031QHLNN-./ÉÏNÍÓKÏgU»°eÑ…,¶âµúg¯ğl{P•Éª´´$#5¯$39±$¤Ô*%RnÕÄxÑúwyHˆ-ÛxZIi~QfUbIf~^|r~
XC¿ã•ÚòÖ§9ç÷rî×u¼ªÛ$=ùTCrN&Ğèø¢Ô‚|Ñl»Z.dxŞ‹{yû·e`Î—/Ö7BU¦%–T¤Æ'„iKéñ›O»¿oß7«yıš0vÿ†ò¾ÕP½™y%EùÅ©É% Kîó.m›^²ö×½‰Ún];3<—LZ½ª0d4Ü5&’9ïê6h¼í¯_Pa•dUX”šV”Zœ¿kŠ§j¦™–yÖe3zşl_iıÕ¦6?ó?'÷½³»vïï„Ï'ß·/Ò•pƒ*)NÎ/ «Yµušß7¯r¤ØÜÏ€ª)-N-9ÿâD{OÑs©	õ5‘{%'N°{j ËJ½øè‚xœ8 ÇÿÕÕ(B«óÖ×úm­	Å?&¾‚/W±+,äğ¼€œïBD#Õ¤š@¹©4“kjİw.ë%€ûExœ}ÏJÃ@Æ)MmwëM±¢ÅBDÁƒ‚‚šE°‚àÅÆtl5Ûn¶ZñîI™çŸÀGğĞ'è(=‰I[ÿîa—ù–ù¾™ßKê)y3•p„§°©&nÇË·­=ÇùÌäQ9••†ªéÛÊŞš(áêYxƒD%]<AŸ–“ÃÔŠg(OÍı×’uTzAÆZ÷Õ®#é¨#JÔÌÓküNc'¶¤Hl#i2´Ej'€Ş´¡y_É†£8C)aq	¤85
íe9cÓö÷HcÃÔÃªktÃÑ  blDë‰©ğÈ2]á^KıîSpD+šäW}ÁÒéQ+6@}ı«¨ğÇºPêè2£GRƒª¢­ìCÛG°Jhø»úÃ#ûc’®WÎA@@H¸àÌ¯W»ı!"‡Ş1­Mk×‚üÎö|ñ9@£{ëÖn	–`j¶ÓVäì@‡´5£&|U–è…zµi®Vìgœ¿Œ—ã¬Ë'´á—ü‡nÇ¨ã‚xœUQÁJ1e×^Ş¤7ƒ‡RiÉ…Ò‚ÔÃV-okÌ†mÊî¦f)ŞDğ,ìwxş…ø'ş„“´…ö”yóŞÌ{Lş?>Ûè4—ffŸ(We”+•"²Vfß?'Áœq*³>‡RT&İ [½­¹Ê„/´È¤Ü¤VKßË4ê”k® Ô´b¥ —¢U!R0BM¶ïAåõx”çPKÆÉ_O§£!¹ bm•nKM´SZêWî5›«°>âfß¾0íWÅ¬™UÉdE§Î#Ç¹ù=îkŠ{ÄÀ¯¾Â‡ £¤	+› õŞáõîë£A‘í“”ÄNKo˜®E·y>h÷èÄhYåg €ë1»Î ß1ıq×aGbä²(-_™‘ªÀ'l"9/Vo­yğÃf›·à€á+xœëdkc¨g2qBŸ[[–ødOõÒûKäœ8Ñ;=ë™Š€¡™‰‰BzQb^I|IeAj|Abqqy~QŠ^z>CßÚÈuŞú;ÊÚÜ/<­2[´’áêÃÉ)ŒÊ¶åZûdûß1Ö)«
¾U¬¯+[¨yjTf^IQ~qAjr	È„è@µ÷Gg²mÖ}{aOúºÆ¿æ(M>ÂØ29Ÿ©MÄÚpiÆ–w—æóº–¦›|t4µî¿Â IÒJªà€ÆKxœ›#}SjCkI~vjŞä¬“²ñlædÍeœ\ÍÖµYŠ=–irç»Íõ\) Æ Ãü«xœ340075UH-*Ê/ÒKÏg¸´oÃNşÍS-ÎuÕ«–•eŠª4„¨)J-.ÈÏ+N)ÛÌ$ç^½w’üÒı‡>Ùü/ùì¥¼D 0¢(ã†ürxœã ÿµÓŒ9e!
’]í¶§‚.õöœÑ4l100644 README.md æÛ°"oO¡Fá	W’j`Ì.‘ÅI1srüP¢t„‹p„›ÏÀ:40000 db Ÿ‚ˆ‚¶QÌdÖ+QúJÖÑ ÀÁ“?13áÃf´½u³˜mêÕ r~-,í40000 envs í°?bo9";2=é”æc*“£,“/ºĞ^&!AŒ©>‹ñ|JŠãÏót“V-t_üéñ0 ÔÜÒ‡ØçêÛíQÙ¿a’à€sxœP ¯ÿÓÓ±'c;€â¶ØAlæãÆí´“n[j‘Å«ÉÁ@0EĞ Œk;]ãI¸e9< “„zEâ6Ó_k?Põ(m/…Q3ƒèæ“AÑŸ"ÿ¥xœ340031QHÎÏKËL×KÏg~êr?Säë¤¶ŞĞ©ö¥ŞS6g éZç¨xœ31 …´ÌŠ’Ò¢Ôb½Î«+#ÿŸùçzqŠZëõ5VÊ&`%¹™éE‰%™ùyÅG­o†¼öÎnïÿûûÌ®×ú¯| :: †¨	xœ340031QÈÉONÌÑKÍ+cø´à©ÎO[çjımÿ­ÏZşÿŞ²FÜ¢¨ (?¥4¹$3?¬òô\…¿§Vß;¯0ıÕƒ¹Bß—Lızª²¸$1=¬èrq÷ùOÙâ9îzl»9CâÅñ_W@•¤—€Õü^r¾ëB*ß¡èÖ´õÖSR¤\onû Ï>Eé¨]xœËÌœ*rÉÎ,rß£§ú²ş¯¦.ó\c ¸cëÖzxœK ´ÿƒÛ•,×\]ÌRšc°Pğ/›Å¿ˆ‘/^+UEáÊ¾ØæÙı(?O¢g©Â‘úuæIb©ÉğF®Ï/³z|£jÀ–Ö à$Ûà³@xœk¾Æ;AybøáÍVŒZ,RZù‰¥%¾ù)©9zA©iE©Å!ùÙ©y
Õ“ï³‡s•–f¦èù¥–khN¶åÈFpõ‚KŠ2óÒâk8R&r‹s¥–”å)!™ÂUË ¶“&Òê€ô)xœ»'Õ/µa‹‚­‚Öä-lv›¥Ùİ˜6×sŞg vópáˆAxœkcmcİÂ(²çŒÌ%™¨Ò¿-‚N]î‡m»»zs£# ÙÁDëO€Î~xœµ’ÏkÔ@Ç±»›m¦)«Áî–:¦?ØH›”EDöĞº-ö"Z­'E†81¡Ûd™™­-¥,¼ˆXŞUğìE<{ñêÕ¿@ü<zs2ÙÖ$(‚—„|¿ïÍ÷½ùäMãı¥×Ke,büøá—¡……\à&Ò¯º.åü~´CÃ[A(°mÛHÏ‹l%=±fªÎ-ê1Êı\ëUö²DSÍDvËö°ÎØiñf¸GzÁ“ä¤"£ƒ“EìÛôYÓ<QÕPØ—u¦…,pĞEøtîÅeyx—zdĞ²'¤j¡;¶\Ëm
oKsçsZºåVEõÁ÷Ê,<2j°¡ÍÁÍ˜'¿J-¤»„Ó3×Ò†woÔ´2ÉR®ÇÚôLÁšH?BG9|Õ®a—Q"(Ç$Ä²†E¼O]å1}É‘bE»±—^VĞr< Ô«óe7¯Â…r«FI^<<ŸœÃ¨•gácõÊDj	øVmSßöú~?I«ÂŞƒı¦e-ÅÃ)pT¸>>Ån/ ’ô€áS%(ë¦R×’÷fïRáGr¹ÀÃéˆß~ `Â«ñëÂ‚{‚É„„ÎİÌNY~‚m©­ÄÏß§'n’ŒÖt™É5á³^‡	´X’ùpç¯áúS@m¢Å?Á1Ò?Œ¢“şLF–¼¿1Sìgñlı3Ìéy>õ"·ĞÂOù±³ëƒxœ['ñH|‚öDÓs/ésê'çç—$æ•L¼`È¥_”Z\ ä§*M~ÈøŠ&¥7ù4“ï/Ó6$Şfæ;œ0}z“ûYÂ™õrS6?c‰aœÌÎå°¹™M‰	,²”£“ ?/1ï‰4xœ»Ìr™eÂF‘gR·7HOPÊ÷_èö3\lRVÂ½GûŞå³·ü
xœí<isÇ±Ÿ~Å˜z±à’ÏtB‘”ÅDU"T‰‹QˆÁî Øp±»ÙƒÙ¿=İ=ÇÎ ÉĞvÕsŒR‰À===}Oï>cßEì’çÙ¬Ï®Dr'’Vk8óS–ˆ8Jı,J–ÌÂŒûaÊ²™`i”'®€6O°I”P€xã»IÄ†b<,N¢¿7ë¶Z_}õ"÷¯Ã2‘fÆCy"¢%K¢qfl<œ²9HßéW_µZß|á8ìjxò~È¼ÈÍ"—ME(€î±áå)‹ÁSÁn…ˆ›ù\„›‰D°,b<¢ƒ]E,=DÈq¾•Ï.ß>²ó³‹!¾º¸bWç§Ã‹Ë·vñöjx~rÆŞŸ;ï?¼Õk/Ù‡wg'Ãs‚Ğj9{b×—'@2Öïn}Ü|!ùœşV»Åt>â‚à¯ïòÌBæR»ÃKíjÎw	‡)Ãe,R9Å_N†¿°ŸF ¸(ñÿI³Ø)r»ÑÁ31ã/à$|×Ï`”¯¾š¾÷Bâå(ÊŞñ4]D‰ÇNá!j<@,5Ê‰p”«Q[Œ2ÕË Ô†+ãiı	 Ÿùpì'!;qáÄS6ŒnEHËêN‡‡§N'ÃN5›²‹0~‰É$ui„ãÛ­m<¨wA>Ş…±üF­W éxÎ§HÊT¶8©l¡Qg"!àîút,õSAÉò˜æÂ_…È\šğjñIó éA_T+HW6M®şJ0O£yì‚}ÉŞç!;ã§™Ôè8I.w5a
á¾å7‰oäŞŠ1¥/VC° ^¦ÏqeƒÜGÇQ‚`Hœ•ğâÆL‡êh·ZÏ1#øëkäúVk–eñQ¯—EQv}‘MºQ2íÍ²yĞK&îş`÷¨G§åìtûİíVKJÙ•ƒ%,‚-|XP2³˜Šmª¶‹3R/ /Y›-`&óÓ4Ç}%â9P*Eí0êİm÷Hh{Ä5éˆÁ¹Æ0O—½à©ï²WÃá;V–T–Î¢<ğØX°<^WnÛ\jxÆê‚ú(*ì"†p
%Ù–ú–ôC½ û",p;Ñµ3G@).Ln‹¨¡ÄÉn‚¹QœùsÿŸ  u80æÄWäTN»ìÊAEdhàHŸ(,ÇŸ€–íË¨ 3¹<æc`âhÂ€¨ ´]b.:@®U#Õòœö’8 v cö$‚%¬¹c6N¢t·	w2à"ü;„ˆFóÒ)oŞù%„Ûl’DsZ»L×”læh4j}í˜Ï×­ÏÌ¨ÉÏğƒ)e©~¨Ïçòjû[Kuáÿ›/ÚrÈg9¤˜ªæ‚è>ñ4~,xıq¨só¤vôC±=öáıÃ¾o-ÜĞò©‹?Ø}ŸÏö¥é't.zõ¸:B,	gÊŠÕ¥aV/`ß¿zòå½ŸâêuÃÕ¿©OÿZ’ıs™ò+>uÊ+ŒîG\ş&ÿÜµşiû¡3?k”½¦5“–¿{üÌbç÷S©<³@óó·x<g«G~[3•X[­3u}i{Íoµ6óÄö¼M8•<æÔ?Ï(´¹è±Ëñ©	¡Mº„t¸R‹~èƒjE!A„Z“—LmTİu*”|]s&«´’ä,äƒÔmFÃt´‚İºQ,:,ˆ@Õ²4ì¤s^R›t°úbæ»³•Úôz 0 !b!>æî-‹ĞŒ(Ã†„Œ °	†0ó…×VêWYIBi>ÑÑÁÖÁVL@O¯*~/÷tã{ÇèøÜ¨ŸÛ_j¤oòÄ?F@éïvN~×	ÿ‹EW|âà‹.Ø	šÆà‰‰4¤ÇhX¿¤ı§Ñ\Ğ·/‰8Ç‰àŞÍ"ñ3QœhãşËŠ°~šÒ•iƒ›şâúu4O–ÅĞ'’0Ñ³(ûx½ıqÍr¸CLÇŸÎ`Eğt -iZ™¨ö^Ñ:µ¬5ğ˜:Å„‘^QGªH]LÔ*4ƒM¸˜—Ë¨‚8p¢Ãiøğ>D}°04ÎyæÎ¤.ñõûÛ…(¸™E¤aäJ6¼zuùáõŸfŞDŒH®$’à!³¿ùp5do/‡*’ÊïĞXVÙYÉ¡Â¤º™®EÒ²åË·v_&(:eÀ})°)‹ÔÑò° °E:IWÒGÆfTCÜó´’„‰yÂç"ÃEå~3Ê@Àâ¤H£\İ¾â¤¸"u½ßğcI€)øU¹“bv.û¼Y-jv–PÖlUã–6j$µ/Eƒ<-+k>ˆwî€=&x Gn‚ŸmŸŠ(˜®`%bê§Y"C{©™«@ŠÆ­ûø¤ƒÃ¥­˜<à¸,ÄV÷Áød¼íîí9ƒ}±ëìºƒ}‡{ãsèíïÜñî€÷ÍÇaYã^#êV¤±Ş¹~®Gi‹…ÛTĞ„o ƒôîaÏŠäq"îü(O$"î²?£œóÛâèµR4RVË&øh hOT¢®Ø£ XĞc2x”g”ÎZnàb 0k#,çU,^%:emı“³’½;¢_2Ò¥~mlH£VÂè1ÚĞ£ÌfFİ´Ö¹lƒxåRn¤ÃH‡™Ö:%¥STÈÃ³ÆS1ÜB–FA³«¤ÒµòpL¢Ë@kßgZ¤Ïà¥R+Ue ğìP\a°ì'eÅŞ’ş:½õ/ğÕ7P)³qÄ6¶7:Ø¢t#Ææ­-×õv·„3è»ÜÙƒC‡ï÷ı{¸'úbg²íÊ™âS»Ko|œ·³¿µE­ˆ¡½ u!9ül,\Ù¡0.pØŸx^Ğwv{€ƒwÀokâìîìì<÷`<8ÜŞhı Uå?tâñ‘Y¾Ô4:W¹>×QNslÂp/‚C£Œ©”•4Ô`I8zŸ”-°ó íæDHœƒKåê»ÁêĞ²1%áHÁ8çOjöUH*Ìd©h“ÊÀ}	ÌQKÆuVCY–S4€éğœ|¯œn²?ò;~EİÀCÿ]Y™aàßŠUZ¶àô8­(EoœßW¤=šLxàQ—Áá&¨µÌ[%2*Srä”è§?Œ{T^ p±Wq¹á`e~¬ ê¨¤^€edL#_’Í|&®â2Û7YåÃ1öB¸Î½¾Ià6X$òˆ•Uó=}$ú™TR¤„vNÒ±å1’‚B¿8õÉ)P;KÁ÷ï„’¢SºŸ9E÷¤İtkósŸïÉÏ=1=·.;§*˜s“É¹æµI•2øYGó2b ^&|ŠªöQ»[¿‚£“[Í»#ÊşYŒW%WÖî.Ê3ƒ¢=@qÛÚ4h€„böæËv%Ñ…ªdGŠ$×ƒÏ¥ùS§šJ¹ja“ÉLı®]:Z«_å.íD›®³Ó…¥v{üoi·ÿ?i7²)¿åİ”wÛù-ïöëÎ»M"¬WÁ_&÷–êMO´¥xbşíÙ¯ ÿîéÚŒZÍeM@“ñ>*Ù1øñÖÁàp«ïí9ıC1pv·c‡ïì:ã½ßßò ‚íó/‹Øû#ïš«)¹",?–1y¡å,BI¨ç ü|øÜ•ş2°‘ÊqÃÉŠ¸ ¿™ôFLü¢rSÚ•ZÀQb7ˆxÙõû—§ııíımEy¿D5af’ätÉ	dO‚¥ŠxÖ òK(®$ígG›!{5|óë±rZC'_Ä|,<L*º4m;•§hÄ+;ÙÌ4fˆŞÜV‘j-v¬2QŸ27×"¤Í"ª)ä¸­ËçŠT¦Å‹¢ŠOÂÍµ-“ûª%×PR‘¼£…k=`­/ŒÅ]M‘mIR»*Ãó€ò±G&v$Be£+ÎJÅ>å¼Pšû:Öå©´É‰hR]3,²“"ÅqÎü¸Èèœ¤Î»`›>UO=]éçh8›ùÓ°hœøw~ ¦p"V4+ÄFí¨*‰2~§—Ên0_D%K"DƒÌ/í—Ø+D™ òFˆµi¼ä¸	i‚"~P¥«j9WĞŒJT¶Ê™c[é —*¯…BÒ7}^+ï¥Ò[a‘qº£…ç R.2Jaidò{sJ©ñÉ§²6ƒŸ„£Üƒja–;sLô¨s£â-YÇ;<ó§"­ÍµdÛ˜ê[áTŒLc (3:YaU„~ºtÄ]«!£Ù²ƒ§F)c:zªÆN÷Åmëã]¬å <…»%aW£›Óú»ùÖ„´RÏcèø÷UëñÊDÄªÂµ1/}[SÜÁê>²€¦NùÂ¨j@©ûK®#©+?“‘jMöº¿Ì…-æîÅ -ïâ§¿¬ûe/Ó4iÍ…˜&<ÅÂÀ_¦K•armfÕ7Üxt€«©G^¾:«pu@ÒqñÍ%ŞŠˆKü·\OÕ+Õé«ì–$Ìå&·_2i³Èb[y¡’ıû$Í·º¶ÂLÁ1úœT,–ªg¶#vÏ’‰Õ+«+(Œ…gøôEèa0Š˜DX&P ¸.•>s(1©0]¹Î8Xí±€õõ5<z!	È:=k…|yè_xy¸ŒtM÷±XF*uEç-ÃZôOĞ#Òçí²B[á	R>­ª³•Y¯W5WÍöS-åıÓ×›oåeAsõ¼º/øy-å‹§YÊÇîı‰”¯¦xa5ÔÅÙ}±lù~aS£†Y\ş“›ÿÌüt?[ıØÑ£LÀ¾É76gû ^µú£I—<4!xSmM©är*ŞÄºàTÈú<³H=Äÿ%·|8EåÕzü°óZÏÕ¨èğ<¤ğUó%ùÊ‡ZĞàÉkş¥IömÁS}n+¬4©uc]^Ka‚	´İAÜJ-Ù{²]¬å98Ï*w²«lN²µn¨ô‚-çˆvëdâ\f³éJ_A¬KöÿTŞãöäp¿?ööœÁÁáğÀçŒ÷·½Ğ#œïôÅşÏ®Avøşxwo|àzıCg×Gòí±³µu¸#x|¸³c4Èj<ù^R8‹2­ÂÌOÙì@N‹ç§.O¤•Çµ”ÉŠî˜Ÿö­¶ŒÊÏ®Ä¼¶HÕÑ4¬2ÉÔÁÊLrz%È°b®i˜¯³ÓÀ¶ÒÉ£Mƒ“&%/A	6·n5JµX”+_Ëe¸êQ¼†çB[&Ñ¿B±ö÷ûTç%wî‰şõ^RëÅÃªÍº= ¦ÑuæXçc¬ àOÑóÖÊÔõêBä!f¿4…$ôlnv\Ò ÷È”©·|Èîm7ó‹ ÚY’‹õÊÀÜhcgiû²_ùºû¦áõê
š¶w÷vö¶·
„©g‘UVÖ0<õ!vŸ2|¾Ø£ËÛÈÃËXTI”Ogú2ÅD=ÈÜe/!Pcú™ù»	d³›šé§ /ò1]l½zæOa÷¦‘ClĞw$
½,¢7ç)¨„Ñh4æiÀ¨a©§«ŸM‘%‡×1~8 Ğ¦„9¿ã~`2â’ò¦lûÂHSÄ‰… tG­í.æĞ¾.ñ4°`”Rİî*nfa®;À”ÑpªU:A@øHº¸ÛêwÙÄgÉFîÜë%yx£r`Óh„(!K,-6 À–Ñ©¶‘"Z÷C*®Õ&8°£L>H~%»6ÖifÑ\µtßŠ…îl·GªhÚÆ[Ş:Èb›€Ji±êb¤{»Ğ³é†“óÆíô%PY]ºv*d­Ã¸Æ+;ù¨¾z0_¾ñè¹ÇçÎŒ‡!ïÑ\GÍMÛà3şøãÓ¨Õë±ÿeÓˆMAµ=LÄ¹µChòXn¼IIøn·K7RşÜ0Û-\á}ªhÖQ7rqÜšä¡[ôm’s;}Áİ[,ÿI3|¥­
,şe ÒÄšFç‰¼I–e6YT-W¤÷SvtLfkóúãõÇñë”¶Ûã®·>²c&‡à)wÕ«º²_N¡eO	„K?»ò7>:/1¦DğÏ‘•Ø&gO~ŞŠl%·¦{#sã¢ûÄóĞğİGû;ƒCk€¾ŒĞ#6¬¾«Ò*{”ã~0ôL3®*Ì5“+Â(Y3 ¥äÈ[¢Dò`‘¥-	!"Ò¨$>jVjÄèÿhØÇ,ôVO^¥cŸÂÿ÷ÄDf¢{DˆØnnùE„ìTåí­Ö‹%Báy@®e»
Õæ‡Î\Ì£dÙv‹n}¡i˜’:¸‚nZXê¹~-ÄÑ­ÊdÃ½Jë¡“5ÖšSi ­£;îéªìB½T1’¢PìIŞ?Ïg4Õ©Lmk´	ˆ°tb>V'p¡mÅˆ¼Xf¿rDû±íîövÇøÜòu7è)˜W’`!HgEjTİQûŠô`Ñò@¬Ú	>n zÿÖï½‘CÛúæŸÉéH:ƒDğÛPP± 4fŒ6#áËâ oàíPœD@öy—½+RÈ0£Ùh×W%`jsdÀ¤}Ãs»†E:ñëvcÚlœg:iÊœ5ƒÃaFˆ×æ!›ğ»ˆªÒeºŠë³<–ÖÉÃBy a8-Õ–6?h¤ñX1áSB ßõÂdÆ!Í¶I¿ÇB›`%9mŠ3´÷€§ƒ(Vµî—WéPe:#œ?B€#	qÔa£wê1#r~F!ÏÀ€7Ã±RKy7£#á|Œ±0©ƒœÑ×Ç˜Fa¡êË¤‘P)	,ñ—˜iXçÃÓ³Óáë›“wÇ;ÌÍ½{r2œÃô·´D×œ='ß_eƒ¨€s,ÖÆPyÏúı7Jo`5¶›èEw¼K{»;}Õ‚uî8´º°™¡oàŒº7ˆ¼UŞıª¹oø§/ ŸÃW-š/ßM3´ş@?½E«ØÌ/RPúÚŸˆÌ§Õtş¨tduÄvë°„É|ƒB„YT©êbYi²LP#£¸‚„áFÏÚãÉîoí˜E±€â2–*2‹]¤gÃèÎK–²˜[^|‘ ¯àƒ¤¬w:$5#‹sFÄ›š¡4šTd¨Og‹1”–2ˆº" Î•Ú$I°¼‡9ÑÍ[ËœEvXÊZ£Ü¨®ÇH’\ƒÒ·w¿ÉËoòBŒô(î.Øç­mF#Ok&’Ã¿ÉKÏeÁµ+ƒZn›OqBÂÌ¦è‡7fU´Z¦ÇùPë¬ÖÙ’Ò›Ğ€^ê§ŠˆÌé=u‹¬ÒØy²¤¤],¯ûô£¹lğ)©Ì–YaÔÈäiÉi%Ä´dŸ™1ÅÊœ§¦VÄWµÉÒ5ôû,ÔÊé]Ş¦ê¬ñ™C_=,Á^ÃÎ*Ó1"´hDe5òeŠTü¯]Äœ°¦2G,Ÿïü$
IYİñ„jSºF¡Ú¼9{öîòâíğjdšNÏßo^^¼>/šştş}¥åô¤ÚpùöåÅw7ïN†¯S!>Tá‡H).ÖhT˜{õáµÁedµ•‘Q%tôÀ“zS	%ı,$ŞÙÜñ€rãèNĞ$9&ê%o+¡Â5zSöÑ>nfåT1“-k:À¦ÊÊªhÀ‡>!ßæÊVóĞU<Ñ¥6y òjˆæÒ~&FZHd²Lié÷wfúÍxÈ…iNŠ‹¦—º
KF…R	©¬æTÑÑ	1H Í01¥¾±ÉìN¾‘‚J|/(lK½¡ĞŸ£{M7…,Èg¥uub Ê—‚¯ |‰h™ÖGÈaiAD˜`ev?.òãıÁ!sb†yrJ–ÃiQq]íW,UèQdyÔã ÜŸ2zmİÛŒ&Xã=ÒüVİv²oÌ~o|ï[†Á¨³Z2“Aò³¿‚±‘‰}½dòŞiw9šz“(Xİi%ÃiLåàô«$‹Í¢Sw]~­d¥A{Ú•ª©½6ò[‘H³î›Ô‘êË‹g·?¼•Á˜¤LZ¡œ^˜AGøÉ÷²Œjİ#´®t•£\;Ëw#©*NúÇBÙä.;¡k²B< –Q¹.óŒ;‰mSä8*¢–Îğ¤ÈyÒ~eš]ÎÓ÷BG¼Ë‘0h»ÿmd‹|à»?xœ»Ìr™eC£ÈÉƒ®z²­cù{îHµ”´a˜ÜÂx ÔDnéàfxœËÌœ*ÂüÃ`­RDå‹¯'>‰şşvå÷— sìÍxœl “ÿ²²65Ğû@ßB×òüúÇsÌªˆ.76S40000 domain ÉøfS˜Õvé5ÑÓÍ=à7‰‘kqj”J	U°Åš=¬ù÷c!ı÷‘ğ.¤Ç°ÕóºÒªÿ®ÒPaÔª“›€4;®xœ31 …ô¢‚d†{İ¯nìJÔu}ò ñfF’Í×û&`éŒ’’†S•Ùª	Š’EFF<b··İ|zr=D:;1-;‘áÓ§ÅÎ§w%ôİ=»ÉAtöåµ ÿ'ª¤xœ340031QÈO,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏgpßôê‡«ëi.©„L-ÕN¾zl[1ôå—–@Ô­ºxoëûŠ{SªLûz÷Ïu Š *Ï«xœ31 …Üü”Ô†%üuŒºâÅ®ËûÄ¬/\¡ĞûªÛĞÀÀÌÄD!?±´$#>%?713O/=ŸAşĞ“•O'1jß]vM_rârµã ğ¦îËxœ áÿÕÕÙùŒ0m¤ÿˆPØR:ŒW7ìDëª¦‘íèğP›ï¸Vxœkcmcà%¾®`Í÷qö	zW¿>Õü±cÓÙqL áF	k†¸%xœûÏt‡qbĞŞÉVŒò ‡l§)x340031QĞKÏ,ÉLÏË/Jeğ57}¹¿ñÙÙVg?úsÉ°¯¡ÅÎªª (U79?77³Hå¥e¦ëU&ææ0,\§ÊûÇñWîçÒGM’Kêd-~]~ÕâãéìêìÊpu•³Lü¹#)Ju®j%ßj)øUâ›˜š–™“Ê (“ªÈ5)öí¶åM<z_¿Íé½h’Uäêèâëª—›ÂĞõeƒÀ­¼¯âLÚ]w¬±¬¿gb 
‰~ò÷Œ³móVRñ¿øÊ·ùı¾iÙd Ş›îi*‰¼¯Ë7‰¼½şÂ¢¸£*öÃqeş¼ä¯?¥íÿü«zµrU¨ÿùù)I¯7ÏX/Ö”Ü%–¾ïã“£'î@%Srò+sSóJŠ^ßnØ®xõó£‹’Ÿ4íOİåô?U•Ÿ\ÌÀ·x£×•B‘ù“tŠ®&¸Í‰»é©‘NÍ++fè_ğl‘A—yïû†—×gİ_§
•®(I-ÊKÌa˜ı”a’€Õ;ÇzVó­ªÑqÊ'œ¡¡”¯—›ŸÂÀÇñ7É5mÁ²]6^ç9ù[u_øìŠ@¨(.Íeø³èÏÄ3·R.¹ë|_‘÷€™Qò\ENb^zr&$f#K}¶ÚİšÁ®jzôÈµ®=kâ îÉÌƒºçâ¬[.1¿zEf°ÌËş´>7Tğ?Ô¬ÜÄÌ<½ô|†£á“>W»¯_2}çd±OEÛÊgÎƒ˜RÎ°ÿrÏ¢¥3ó¤XöŠüšu5šUj×’¿ ï­9ëƒRxœ›Î:uÂF‘gR·7HOPÊ÷_èö3\lRVÂ½‰G5‚üº™²OJÚú½gÿê7Ö"Œ{*M€@!9?/-3¡ëóÚc±oN5~o8Ô$æ®?‘o[DAJCıü¦¦mgR®	hşòºvqÁƒ“íNÛ²·tóŒÜWWåÕ±ëê¾…èJÍ++fx»Á>)ßR‰×ÚÈöåfg’™´&/fœ/’œËí2Çæ–Ğ[ÎË?ä]Úw|bô–šÆ¤+ÒWÿçåGƒWî\j¿ñüÕí·Â7Pa:¢xœ340031QH,(ĞKÏgHaPT9báêäìöwÁš@‡¤g“¸ ª y¥xœ340031QHÎÏKËL×KÏgè1\duµ­s±ï§®ïa^‡Z,„3 ô`“î…¼)xœ~ ÿ¶ßS‘š‹0“&ë2“k“¨'“9“&428“\6.0“ß“
0“˜\ v3.2.2+incompatible“O“Š
“!k“Åñ“a“¾¾xt v0.19“]“É±“ŸxÄ'ní€xœM½JÄ@…ÙÕÈ*X¸,Jlì2Ù$ím}‚Éd6˜dâLÄ§X˜ÇĞF[ñì¬Ä'PßÀ‰6r»{Îwî¹ïÛÏ;««£KÁJªóÓZ5°PÁRÜ´æÆïp„SLHL0q,ò$Ï³$f)''_£+•”·°Ñªp”É¨†}è÷!À‰='s¡»ÆğJ§w.ƒ ­ÍB´e—¦*hš%!S™¦ƒ¼ Ø¯GÉãËØí‹š©Ú”š÷œf’ÃJu†·š6ƒÛ3of?Ö¾•hYÉ¥,]TÚ´ZÔÅÿÁ!Á$p£˜G1Ê3{áíÚ×u:Õ¢W°«…á¿Ø$ƒ4ı+×,%-†ƒDn}`½§‰®#‰í½·g?½·	ñ{â@dï6æ?koùîJ…¸xœUÒ=lãdp%-wZ® Òê®@AœŠ¡qœÄ‹lçÃNêø+NœÄÅñ·“Øq>û$$$/ F¶`+•X7À‚ÃÁÀŒ¸…¦A"lïòÿ½ÿçÑóÑİÔO_§>{%¾MãÉ[é7/›ß'<p˜†OYÙÓ9RMqÌôİ~½>i˜­b'!Ò7;ÚT/ÊeÇæÄ|…:¼¼õ\ª ô-
xWwŞÛÚ†Ñ¥ĞÈÎ[5Á5G¡dt>F  RÁº„Ú1ÕZŒO3¥BiÔJ@×l)øe‡'KLÜÉÎ&M£‡6µÀ¦VHeÉŒİ›Ğ3´ËºBy.…K[ŠK¡­R"S¢Ç e‘%¹ºîÖ¦ıš 'úÏ»GW!3c‡‘y)×|)èB¢0…ÊòN`Ú]æG2®™4ŒeÄ¶¥P®3ÆÆÃRhë«øÂ_N1µ51†1MĞÓÖ™Aö4H3Í@‹±<äÑóF–_7Sp¢oı}¤¯í¡Ó;zÚR)ú12-”#ƒŠ HÁ UºJ<ÖûÃNW—Ëd‘“«´»¨ALrÿÚ»àxïcğÎşMÀíš|yÀ¦’óëG›Yää²S÷Ã<Î°AÔ"?ÛËkq8°ø,‡4¤3A&_€4ä2àüğ @‡ÛøEr­ÌãT€”jŠÊÕsõ ç¸á…’…Ñ<Ü«Â’EDQŸ–jsœßØ?î pøIòÆ“Í4ÀŸÙ÷>Ln?{o°Ç{`÷ø}°=~yş/ğ+¼nœ<”üğâÑ‚¯êÒÓ±Ï´JÔ ›…&V€ôl>‰2!=ƒr¤'U)•$ú~sÒ4ÁğÓ)<¡³¿?˜CŠ«x•,…Vi	v.?/PTG"[›Ïò}uÌ9ßl ¼	£|szıt™[›5š”é…Øe£ì8DM²|$6Ë7v‚ÃdØïu““/^RvÁ#Ô•äöÙfAWÿ×i‡ªEš¦ÀgÔTs5hŞög>û°ÜqËÍ0†»ª>o¤Ğä»Æcéd.`é\öß¸ÍaQjU]Xu˜	M©n³\¡ò^I€Eß°[Ò¨Q¸Ã‚ß¤ƒÓenmrV®—ŒHR%1¯üPä«Ì&5£hÍ+ìLÔ}¿g)ö î$Ùæ‡W§Û£]¯©^1&\¢—óÅ8h9İ£3ÉĞ9ePk,,jƒG_}¼şÚÕSôÿË3†Oˆ‰#‘T½g#Z\ä‹®ÏMílÆn–ÛQØpÚÄ±îŞÚMŞ¾u/¾R? œ¶üÙ!ıhl‡@ï+†kxœ…Ğ»nÓP ÆqEHHÀÚŠµ»Qß$†$®Ó¤¹ø’Äq6_}\Û9'¾'‘x…n,ˆ7 E‚7áIx jÁ"U¿å§¿¾_?[_~´¾½xyúôêİ›c€ıâp¸*™6Ûî\!æ}­ïù¥£ŞŞGõ+MÊ 7èQ–ÁZîàÆĞÇÓ°óáuæ¨pÚ.Nè ã †ô#ïaµì5¬×;}b‚rES‰¥.ys$Y^ÜÕP45öòbäÅô×Áıç‹ïGœ†qlÓt‹ºG!lÔ·á¬ÁÈœÀ+óTäR 2B2Î„M‰ò­%r¨_^+R´–%¨öş«|’=‹ùƒcÏpÜ›nXgKyl«L³	»Z¤:T˜GœÛ‰Ÿà³,ÄÛ¬¡ù¿¿FddÍ	C0ÒDPƒºG–R–fFÕÍÅÉy0ÉE»8ì´†\=gå*3:ØÙw
Å"¡+½–sÔuL£ˆ:»Õ„…‰+Æ!Œî¿^
§ôòwëãîQ W0¥á¶tñÖƒ³äQĞ|†<L"_V2G
 éò,*LÊyåt™ËÌº¥ôè…è»Igÿµ»®»ÑchÄŞñÌB%‚V‡N6g×Sg,ÏšV´5§õÛòcÉî²à›‚jxœ••»¯#WÇuï"" 
KD
²\eWBÂì9óH‘âyÙãÇŒ=öØ3Í¼=÷{Æn¨h€QÒ®DEC‰ÒĞò7l…Ò ªØ{³È7AY­N{Îçûı=ÏşzõÏÏ®şü‹«ƒâ[[#·Çq’B^òÜõÛ²Êâ¦†oOç9  €ò9e“¶m’„E;ÈÍ|¼‚Uá=¼ÏGMìïöƒ¨v˜+#HWÊ×h7¶… j4ø“ïz~¹­Ì[+‰ ÿ©zo,{º~%öY½±FÖe?’š‘E¨îŒ±Lã Á6Ï¶ÆÄ*7ânÎßÉWŸ]¶Nìµ~˜@µŸ—•–NİÔà$‰yBLyÅTV¡ËPë@Jsès&Ó—TÒò#EL\2
hhÖLDc}÷Â.÷³®m™ÍjŸvB»¬œ€ÚŞn‘fì7kVƒÀy¨ß~ô/ÅJ+¯ê(ôœ¡ÈF"#†jä¦F¾Â-½yY¡¶˜Ëy!*µ2[’k‚\°F”ÊP$Ë0ì\ ’#øÖGwÿ}ô§Ç?¶’¸ØæNífè@QRN™éÙ÷ÉùÜH½“VƒtZÈ$Ÿ«1©"Zó	OŒJ€jH›Y“Ò¥E=ÈÇëØ'ïè+óu*° F,e6*¯c¶ØÚˆMÜ‚ïÓ3ÁÂ"#7›»ßşğ×oÑğË÷¯ãku37ÖfSµ.‡Çõ¼ê/|ºb´fÎÅZò!['Éİw~\šW±Óøv¹½o5@œ)<¾°¤ié7uX3xjL»¡S,XÌ÷¦&øÁ?å–Xª“¹uüË‡ï¾¹p4²t‡„Iço•ı4µ(®ĞŞÖ)±¯‹-ÀÑLM.‚6gÖñOŞşJLÂ¬#ÆÙã`ÌñÅb‰£l5Z£*Eíƒ6õBÎ3­B²¾$<>şêÉï‘_:1·•„I^”¹{—# E Bãà¹N8ÛæY“@Dt§ |4ÇB<ŸB˜À»ÜÚR¸a
¸Û$_ã%™?(õ…æ›‰^ÄÂ¶ÓÁ¾©5)Y‚°2ÙI™ñÌÜÌ—˜Ì6ìb¼äkÂ‰Ùæîû?yñèà×	TÅ~á¼ÂnÉ3e­Úu°¦*~›ëî„Å6{;‘}ÙÎ€ˆSH£eŸf¶zAş5Úe)$tQçÓŞÆŞb×MÜH¦ïKf¨M­Ä¢8w9Ì+v¦Z-E?yúÑñ§¿9äUQäIQ@fhX›û¶Ñ}9¸àA¡{JÔz=ŞVÎ‚†O¶]˜i~7çt¥eW{ßE¼^>Ü7…µMÂp¥yâ–fa9T£75zÈ[p8£ML—F2—{5ìƒÙé’2Ÿ$«Ô÷f§l¤1Á	ÁŠÅÕ7@_xWöY³¹´S;Ñ³½Pİ)#‹ÕâVM1ÓUOõys‚_<ıéñoO??®½‹Ğ÷»¥ÌCinJê •m.çüF§¬	†HBÇ#Ğb>ÑÒì8>Ÿİè™‹á¤š$œ?ÃvC)¡¸`™i¹8âÉ¹IbÑª—X¦ó‹]¤.@OãaæÆyQ·àLsp¨nÇ6”o
w1šª0¼rç,9·—Õ ÙŒp <èÍô5¸Í°å­Êv³h¢”¿t'=¹ÚÌLfDuè3ë±èì%4àhıÿ@S74¼û??Ñû]o%¨2;Fê„2ìÜCØ^Á’0¤»iPn\C#­ÿÜ…Ç©¥¹b<Rò=Æn´ÕFf&&l.7¿üşvJ‹Ü¡ı¹f¼»?>Ó¼ğÓÁ`ø¹›¦Ac†‹»ÔñV½ûó½ëS“ÜÿLæR\]è´!3Êë»3×¥jÂ#,œG­¶0¡x´éÿşé×y7åß½>ÆÂ·Q„¸G¬ù¹A¬ĞÁÎ€˜­Ôz8o?…‚ÀĞKÅC­ÖÃmÑ`}õøoşúüî"²…G ÁIiàûÓŸÛÓ8W"€á.EÌ1<)=şRüÑ3xñø
çštœi¿;¼VìŞGµ^Ã³:ÉêuÛÓ‘+˜=bz¤Ô­íw]B±K·ç:&iàİú1ÔQx[¿â\„f@ÒıÁ:j¶q]B;f_)/8¹%6¦–íŞšV1ˆçÇ¾şáİçÚÏ¯¿ £t+¶é€ˆ!xœËÌœ*²Ébóñs2aW–ÄsÌĞ‘( ”Ré'xœËÌœ*R–Æ³%ìßÑûŸ±¶[Ôçm9ç; ™Šï¦axœ àÿ²²üı¿˜Ô7EÅd9¹lÃ©óR‚¨ÿ“"Aqªxœ31 …´ÌŠ’Ò¢Ôb†ë]ZFW:ëÿÎ\îv§–aÃö35^š€•dæ•¤¦%–dæç3ˆ6¶‹jşø}-¡ı½änÆÚïúË wf !¨xœ340031QÈO,-ÉˆÏÌ+IM/J,ÉÌÏ‹OË¬()-JÕKÏghoSv)j	rß?MÍh‡Y¿,ËÃg 2/¸²xœ­VKoÛF¾óWLáCc×”œ¸§ ( Z‚c –‰)µ"GÔÖÔ.»»”ëşúÎÌ.%YÎ¡zÒjŞofx×nué,¸i0ËŠµöà°µ^ë¡´&(m<„5‚·+‘hÂÊ:¡½²­³bYvvö[§›êúpÊTPaÛØgpvÙù µm”©aÃ<º­.ÑŸeÙ/?ä9Ì‹Ñ¬€Ê–Á–P£AGÖ+(î® mPy„GÄ–¢ÙlĞX£CTÓØ'P=»¶â€òü×hq|7ı±€Éø¦€âÓÍæ“«âænz7Óy1a6Ég_¦½Ïâ¾ÜGÅD,d9ü1ZÚ.HÚ)ÉoïNÓr¢å‰vÊ’ó Bç‰íå!´®(FS"‘]ÿÎ5† ©$ì(Gâ×‘’'Êi@r÷”şê45Ùx{ø?ÉÌ1t-;æ_±^PõÉÑB|%ÉÏÚ$z_‰.M‹Œez&ÎÕİDN™=Ç²…ÎvnLÀšz¥­a¹#×{†Ä5µ1c%úØ£Étüõ<;9Wcˆ“NVB{çÉi–™’0Ù£FMsDó°ağ®¬xeÛ”ä–â¡Ä<hÃ¤¬÷0€+Š›Ëƒ—ØF9±±DX:ÛÕë@'U@SkƒèX) Ú$“ˆ¨£ğSDœ)/1(ˆ€‹:;Ä	zÇ*¨%×p^®q£douê5‚Êî‡Õr°©¤=cU©óñÁuå¹w/ïum`Œ%á‘3dû–k£KÕ !Äë
¼‰ÎŞÂÛŠKV¾k{q¸w–vƒÇ½âÖ¶¾Wœ˜­vÖˆâïÊiµlv’h¶"Æõ8š3¦À‹¡’BÑN{?øpùÓ·wëZÿq8Œ XWÉèb ®F2qYöZİXƒÒÃƒÊ™µÕ57Œp&lkTÚQïXÄ®!ú1Ë‹ujÕ:@)Fw‘è°î–š€a¥u¾VîYUš"Ìeæ;à‘dVVğŠÁÆ³ìÃ@ºJYˆkj'šŠ ¢Ñøß¨GáEËÌ:“ä÷­âÍîXËuæ¤¢¦œ•'MÎ¤ôb‚3Jº`´–è?_\\HÁj×–ÇÌKbà«í TJš»ÑÿÄz³€‡å3éÄ3qñ©(îîïfÅBŒ.®g÷Wé?@f› ‡&­K
Úrb‰§?çU!ãè³ŞDdú8y–O™Ûİ%tÒ¡¬TAâÂÍ>ôêÑ>0-rqÆ«<í¤
ß`šõ{ÅÙ3@ÔVé†$Rè¨Ú”MÇ—? ˜zc[>¢[é$ïı×Ñígù;ˆ¡ö·Eb•ë"á-µ!¿!\Q=7mp•ÌõûtA'`H¤Å~âö0ê;œTh¼ş[¤¸ï@%-9Ñ¨$H€×+ÅwC½»j\ª·€Jì¾ên1OÉQ€Nï³ÕìE‘<r/9ŠçYî°ÙG’:ôıKşbö7n:åß”şàÆ-}–D$ÉWçnt’¯Ã‚¦ĞœCßZ#ØŒæP¬ûiKèğzµ‡¿ÇÈ!>dÈJö/@Ó¥xœ340031QHÎÏKËL×KÏghÓœöaU]Ø·s´ØzUX¿œŞì ö£Æ¨xœ31 …´ÌŠ’Ò¢Ôb½Î«+#ÿŸùçzqŠZëõ5VÊ&`%¹™éE‰%™ùyÅ~YlïšºÜùü%„ë¦­Ó+^ >ûë‡ˆKxœ‹ tÿÒÒÚÎ¼—ƒ8ôPæS·â0°}‘î`ˆÄåw”~4XUƒĞ.‰›‰.¯°“œ“AòP,Á«õbòò»lÇ|»İT“³ğBºôf…sE©f@Æà9Ä-ƒí¯‹“*“pNÈîãÑû˜ƒèIeùÑ¹qkxZàAë€xœ‹ tÿÒÒÚ÷èZ
ÊÂ8ËF>Úfñ~†e‘î`¨1î¯FÒàá‰Ì	å})æ´ª,§“œ“Aõ7®İM1÷[<…UÌmİë'Š“³ğBÁi…:’¡WGgt7¯ë=“*“pNÇ4¥5YúkĞğÇS'=³´€*‡É€>ro†ı~xœÛÀ±†}ÂÑÉŒö›;W0 /¡‚f±#xœ»Ã¸qÂ6 	äÛ¨	xœ340031QÈÉONÌÑKÍ+cøÃ½ùËŞ†=F7ërƒ&ÿ½õcÚÚ	†EEù)¥É%™ùy`•‰ó˜u¯v5¶½ö÷|=û)Ïòé1™âP•Å%‰é©`E‘_Õí`šÕ»zyÓWşÌÕ‰Ág¯@•¤—€Õè~}+½i}Ã¾€ ¹‡ÔGT  .Ú@åî…ı?xœ{Ãòe‚kN~rbÎF©½ŒÖäÛŒ
PÖFk(ËI —ï²ê…ılxœ{Ãò†e‚KAQ~ÊF©½Œ`ÆäÛŒ
ÆFkÃI WîRá…şxœ{Ãò‰e‚(‡‚­BqIbzêF©½Œ¬`ÖäÛŒ
PÖFk(ËI ·ÛW¢xœ31 …äü¼´ÌôÒ¢Ä’ü"î¿FÖ8wÙdP—è'Æœ#<Mj	XYJjNfYjQ%ûóÏSÔù½Ü/<œ¢t³Y‹W`ËO¨’üÜÄÌ<-ÓºmUÕÍıáyf¦Ÿ'½nš÷ª $Ÿ!i‹:?ÇşDKß[g™¨ëe¼'"›Z‘œZP’™ŸÇğz)ï\UÃµ7|?(Ì˜¿ëËmÿ›§!j²ò“æš—ÍÜüúÒá—ß0vÏÓ/„È¥ägıQÉ°Äáà³
Í§ºk§MÿÜú­œEàODQIjqI1Ãßı3®˜»öM±Ü™sx¥Àç ¦ÿ!ò¥Å©É‰Å©ò²‹¿Nkkn¹nÎ[Ãßù«&Ë  ÆÙç#xœÛÄ´‰ÉËÄ ’óóÒ2ÓK‹Kò‹Vîu®¼•ãøñ×üu½»£r¯%oõ€(KIÍÉ,K-ªd˜1k³Öæ«¿åşM8ôÓëÃñù¼.½®ˆØ=ß¼z’çá·ò?r[Ú—6¡ ¹/Ó®xœ31 …ô¢‚d†{İ¯nìJÔu}ò ñfF’Í×û&`éŒ’’Áík¯Î­>ä7)ÖOZá:³ˆgsD:;1-;‘áÓ§ÅÎ§w%ôİ=»ÉAtöåµ ¿'Y¤xœ340031QÈO,-ÉˆÏ())ˆOÎÏ+)ÊÏÉI-ÒKÏg¨s	»’Z·Päí6¹}Wú×2ÄĞS”_ZQocï¶èt¦ˆ+Ó
ÕOgçÇ®Úó iœ'Øá‚Û xœûÅü’yC	£xª{Q~i†R~biI†’&g5'gêä™ŒŒ©“1ê±sqÖrqÕr > «xœ31 …Üü”Ô†ÿåç£ÍØÂ—?y±+{VoÓÇˆşZC3…üÄÒ’Œø”üÜÄÌ<½ô|†Ä‡ì;ö&Èmùº¡>Ãù´V% ƒj xœ340031QÈO,-ÉˆO)É×KÏg8¢¶åìÔäuËÚw„×ÖV
.Rò`ó6„¨+-N-*†©3->ŞÏ±ê=çáj†?åÚ~²÷ —JÍ¼‹xœí˜Qsâ6€Ÿñ¯Py¸1bŞ™ÉîZšë@è+ñÙÔØ–+ËwM:ùïİ•l,Ù2áî†‡ú!´ëİÕî'iå<ŒÃ%<,å~.¹ç±4çBß·©z%úÇ4!Ã“ûòKñt³gv»ÅK³Éß¦,üVÒ4OBI',“Tda2Q¯Ob†,›¤hl¾ÃPş¼ƒ¾†	‹CÉ…hó×W>Á?·•
ãYËÁó]B'eÉb”H–Ò¡7ò<ù’Sò«39ƒ¹`¯êí{ˆqIÿ.i!!!¤¢Œ$ù×(ÕG|‰à(Ëväé¯‚gÓá%´7|òh€T­5–4f‚Fr-XKCT’M)˜²•0šÉÅÜaKI60§'ïÍó¶eGnNMhD¹ª¦Õ¶13Áz½˜ÈMSí@ûÆ™*K‘‘!È`ºi
S¨†P8XÌ§¤v4†7øóÖ…˜ã3‚şSW—v”@êØhB=ğÔ/®TµC¤ÑøÄhûì%…F=CãêÄ8<:aC»àuƒVF˜Ìå?Â¢øÆE|‚àcü®\¹)mWVã¨S»iëäÕ8ê¬"SÇ*(pÜEm7öë…ÕkZ-»R4ëú^`¢á&ïc[—ñà“âÈßçç×KéÑ kü:JWÊíûˆYÒ­ Åş‘?Óìäq~ÓRç¸V¢D™ÕYd¹ƒ¼JúB­a0åW
âOãp‘IÁ‹Îr'…š*71ª”Nş7h«]JŠÕÍ¤-À\Ş¯/w 5(ô u)VuÎáBMü~ûŒù2¡¬
â¨Á˜`÷e	ğ/Æ„ş“ƒƒb‘ôNÔ.S!bÓÚÆTêŒßÓ;«¶¦îy®LÍWIÀ¡µˆgrJğÚ<pëÇ{-GW°w½`¥Bÿ,Ê$ñá¬rUj@T½ªB›¶|V/kãub\¾ƒYûjp^
u“ği„¤iıx+SjÅkgEµøß¼Ûê¢ürG2–`J­œªÁÚº;:-´ &¥LV8›f:ÀÌ¢ˆÅçÅğƒsŸ-ZÍÿa9#£°bXé¢Ò¾,`ê/ëåâ(=m×=§HGï'ô.†t…*ÏgUIyƒàÑö±G.¦oÙ”gÚÓ(^›U“Vàh2!Æ²^Ò"çYAIº­q	›®¦ÂÙÕÔ`z6,ó”á7>ù‚¹×´ô5ßM‡ıñ€#<ØÁS©V‰Ù0»iêi¬WÁŞıì;€5-F³³igÑ!k’8‹$ûªûÂyÒL3Tã'/±v†›‰=ŸmõSß€lí&Õ}y¶õÄwèH ¬˜>ëİÎ!÷Í«ç\['mo·4&	ÛR\iz”{Úñˆø7 ÆºSéÍ¯
OO‡.îmÆğÔÜÙ›õ€?®Ú:¬Ã‡©]Äˆ°³ÎM»+ë„êHºŸ®zgsV9ÔõnSÛ³²kµI•)kYİYúzÊvWT'ì ÿ<3™¼3xœ¥RMKÃ@=ïşŠ!I¤M+Ò‹7±
‚Qê¹ÛdLW“¸Šÿ»»MbM«Xq™÷2o^-²GQ pv9µÄ¹¬jÒbÎE)saICTH»t‹4£jTĞV+…gØR$©ˆ³>‹ŠGÎÉ<â	çöµF8Ó(,ÎjsƒOõÁXí2oœD‰
CJªæ†ÔIäÚt4çì¼²}
†tÀ¯…1/¤ó-¼nÓòÎù½SÄ™İ¿*»f3làSmeV¢GâPk¯‰ŸW£uZÁ§Ni÷Óíz¥¦Ã€³’é…Ä2÷Ú2v°FÓnç@ûÂ£HùVú
Ua—ññ &ã$`É/å×zı©öd GãıŠwbÿ§~²ëS“2Ø·Çåš®Jg3ÿİW†ÃîØÇÇÚ°~4Ñ†ÒóÑ¿À§¬xœ340031QÈO,-ÉˆO­HN-(ÉÌÏÓKÏg(Óÿv§8UÅl[Ã•¹‹—p?^-hQ]ZœZTŒª:ù™À='õ­A>÷ÿ§‰êpuêş4 ]#õªxœ340031QHLNN-./ÉÏNÍÓKÏgXÄÚ‘=_Ó¿àIáì”ÂğäÅÛ_rDB•––d¤æ•d&'–¤‚”¦˜&½|Iè’å«şÍ™;m9«'’Òü¢ÌªÄ’Ìü¼øäü°†:÷¿ß›e/¥Â¿êĞ
	‰
/¨†äœL ÑñE©ù •}…Ë—~zç1ïà×³·æé•|~/c U™^”TXRYİ–†¦oÏí„4ƒLå^^S,rÍ—8ş×ª73¯¤(¿¸ 5¹¤ï‡©]TaŸüã¼Ç•c|Ouæé¦CæƒŒ†»¦[Q;åAƒä;©¶SgÏòN°çîL„*,JM+J-Î@„_Û­Øy“-….E¸å‹	³a=ÜaS›Ÿv`õêÕ'çÏÊ¡xI5÷›ÔÄ»z6gC•'çı³wöõÔÔ%5ÍõjJ÷‹OŒñ©s
‡*,-N-)¹·KrRó&Û8)›â^ÆÖ›B…ëö Ù–¹Äï&‚ªxœ•’1oÓ@Ç•Ğ¦õ	º´*ğ0B²ÁJ7†ªQE‰Hi
‰QGc.¯é©×szw.FUÕ™¼1vdæ+0ò)ø H`;i;°Àvïîıÿwÿ»o_®¼ÿşáÇ=ºVûX‹é´¶t²#¡¢Üã©²˜ÛæFÂ÷F:ÍÔĞóPBúŒ9++ĞF‰ó±Ğ8„„s4lº‡Ê0ç Cı:€V[`óæ³²ôÜAØFpõ·6§’x"í'a?NëW/À¥@e›¶O§õÛëånµÀuáˆ9ô„Óz}‘Ş6–NşW8*3¨c1„Î zÏ»]×gÎ1svömó©ÊJåUà2s +lyq·]£ğ‚K¿.9›Sæ$8'ÖZĞÛÚö|·
6Ì‘gE²v¡Â2' µfY{g64šyG«³İ…sãQÙ÷‚Ñ×ÆÁÙèÁ¹ŸPÑgô»qış_Ş ìGkx%ìnµ;Öé¡}˜È}š»I;³@bFRcşÍ2Jù²ø.•ÀêD™„[‘*ú9·HŸç—ÿ ]*ÈRê)‚šExœu‘ÍJ#A…!1MWˆ?hP’ñD:Z…a M·Lƒ(DEp£Mw©¡;VWbœaĞpa¸Ê,ÜäÜú¾Ál|÷V¥ƒkQn}çVSÏÓÃwóº…‚µD¡=W¿.RÚt9”,/ƒÍ¹6İZàW™pæ‰ƒª¦iR2ø¨ŒóˆÇæ»4
= x ‰ŠT^!go1áo4ÄyÄƒß®¢°ùltóJ)<`McG_GMÏ®æÜğDz.ÌJRqWÏeJ²ëã}jÿ§§ğMÿ—%Êûy`'•Ç[r„¯™©±àOP"½ÀÏ2ğèÒÜóÜPfBú¤¦c•ä.j¹´ª7tfv0¨6….`=“êŠµü÷ÁğÕ/Ú$Î~s5™Hƒ‡èjK%ƒıÛÑb5&X_.àwû*¾Eâ
Æ¡3Ke¾P}op,IáY1ùUøCI|QKô*˜
½eÙÛö¾d:‡¿ìªòÛË0¿Ú%N(9.©Ø&Ù»Å<ãc>e&Æ«HIbPaÒÚ;1Ò×{î‚4xœ{È1‡cBûÄƒ·Ø“s2SóJâ'ÏfÜ±9†i?ãd[ Ò¹f¹–xœ­UMÛ6=K¿b*­˜UäÚÂ—lœf‹t·±SôWÛÄÒ¤LRk;Åş÷?d[ŠƒäPÀ€es8ófŞ{£–7|… yçÖ3lµN›C‹M«ƒ2ÏŠF+‡{WĞ#£õO–NÃ·3tş˜…ZÑ)ËóGnüİ/`jÌzäR,ænêºÎ³ñŸˆ™ë[Ü•E:ëÏ
æóQ¢ßĞÅ`ÇĞƒÛ­Ã\-*Kî'¸iİ¡¢×eÁ­¸ät>W¸‚•v}¬pêr‡ñÂ¨@¾ìT¥iá¹9NŠq•ÛCšU}¿«Sˆ=ÎˆAªØ7ƒÃ¬fí9ØÔ½ Dê3Ï(`Ti2¢ğy³80mMP_Ç¼GÄ¬%d=åÁÍåB¸ÖY	wü[ìÇK“‹Ú:ä†hŸ}Ü? qÜQ³ûaNyµ(ª±°ò§^%ç-úëFà#ÚS]½4%RÀğÌß¿û:ßã!ig‰çDëtÍzXÛ~³xf·ò}‡æ ¿“óé»éõ‡„ûÍìî>Íßo§³)	áŸ>ı½$7½³A]ş:CøİÊ ­ç[¹ıª©Ë¾FÎtÈ‹şÎÏF ‹ğ¿%gIŒ@‰õª®N¨©u…šd`-ÊN…'4#hhÀ#ª¯¥¶X’¡3¿!R?ÅqåÙR§¸[šbÕr„>ªGé;óÁó†«òÇÃ~ãÿZ–.)?ğêsØw–ğM€·-ªEW;Šuî7¥§2ÅsÙ®ù=:Ñp)Éœ~WÖó¸S–ŸŞ¶¼ÁŸ(ÅFœL×·›P¥õZÿ®…:")€öcTş™õ Ycó`ıÚàCÀGşîv¯µLŠŸ·R¸‹›¤o*Š(ìû
wÊñR†œFnhÍÃ6ØÂiBÖ)k½ƒWĞ——½—WmÇîº¾ûëöCùœ]0XseÀ¼ê„\„|1ËN¸5´’h\k¹@Cş&©!'ßÇwG68¤¢z‹•½ò+Ø“ÊXôúæjuÔ•—óyšâi³xVÀ¤÷p}ã4/ÅÕKa*4ô:ûM’oĞ¡ŸiN¾6˜5ös5êí<‘BFEVDn¦{l:ªs9V5hŠŒ	ÿó»i¦wå6î¦4—äóâ_UK.½&¾ôK´’‡à$)¿”–ä’ -6ÒÛ÷ŒQrè6y$¥%xœ340031QHÎÉLÍ+‰/-NMN,NÕKÏgHw´ÿpüÊÔY¶Ş»1/­Fì—ï*&Cˆâô¢D Ú’Ê‚ÔøÄÒ’Œü¢ÌªÄ’Ìü¼øäü°Ş×M²vÂ\’“DNÅ$^zãŸl^¡ˆ©jgrQj
ÎLÌ)é¸f‹Îe7}‚ì·~öpŞL½‰ÅÅåùE) ½*ŒÓªÚ3÷î=±=ÁÿÏıNº?SGQjZQjqF|I~vjH[¹Ö>ÙşwÌ‡uÊª‚oëëÊjŞƒjËÌ+)Ê/.HM.)´”9±5óQ’Ö®Éë•”L²¨‚*ÌÉOÏÆ|âuÇÔ)Ó.\Ú°mÆ¢b½¶×» jòAa„²FØò+/¨e-Øÿo3sèÆÏ
g@Õb8òà‰bæbºç¢fí•°vO4Lm~N*8ÄÜ®Ûf-y#À!t
y\ô¯şCz½P5ÅÉù@¯#ÙÍØPÙÿıÿgÛ¥-®©>kF³–ÀÔ¦#¤jşÕ…
¬±q‡^Í™œÅßºkU­ÕF¨* YEÅÈ&:İä(uÖ|üñàÆjœ›Ò÷%¾}Ğ Ø ñ}ï‚Hxœ»Êr•eB¨HÎËVIµx?ÍÜÿ	µùkxÜó×mÌ|Ã Æœªì5‚©KxœmRMo1U€†¬q©@i”»ipÄ·TQ¤d(R)(ĞGË™M¬$öÊë…¶€zGBù¿ ®ü
¸ñøØÙHmÒ^ì{ì÷ŞÌûuùı…û)ãc6DP,7£İc–!!bš*m $A+ipÏÔ]ˆZ+ù(™úƒÙ—²»NÄp¶ù}öµq6g7ì‡Úy{Pzr´ZğPë|Å&bĞÇĞÈÍn¿”Rœ~µİÁ×a}Q zQ®¤‘ˆ$—ÂœC#/XGĞv”Ì%cuu€5“&äf2h\ìM°÷*®EÛœc–½Pc”}ÌR%3l"xC¼„Ô~®´JÄ–Èzw~›[sª1U™0JïÓGhøh€}¿zü&ğ‰@ép¹ØßeZ½I!!ƒHìÇêÛ3·¨%µO/Şm
ÛjB‚!8ušìˆeÓ5#Ñ˜Ù÷JÈ­² ±‡¶?Ö.sÖöçÚ[«®ÛåKÁ
]œ Aû­íÀ¶ª­(8!.t& ÏS-\‡Oböº‘ıS¾~x×ÉÔÈÎÅ2ÀxæNFÑî%uK3ğºæÆìºA9?œ2+gÑ¥æ¸ôx{ùƒÂ£´ãLa;Mi>õßÓb=öı¶HĞˆ)úçÎÕ´ƒL£v™Õ¨k4¹–p*m)&äù#&şâƒxœëâëçÛp†…5¹¤BGaó;& =¸çøBxœ›Ê¿cƒ.#£æä,¢“k­&Ÿaô´qO-Q(ÉHU(NÎ/ ’%E™yé\œ`BjQ‘‚•­Bi²PY0HL#¹¤B¢X“kòD6áÉû˜å¥òRË““S‹‹Jò³Só¸8!¼ÉL²úE‰y%`nHNƒ‹“39'35¯DÈÊËÌÑQĞ×WHÍ-(©T(-N-šÆbÏª(È,J-VÈÌ›<ySfÎd–µ µüAxï	ÍYxœë<+0AC)µ¨(¿¨Xi¢\{½&WYb‘‚§¾¾‚kQ‘sb^^~Ipj‰knAIehqjQ^bnª‚'ni[ˆ‘z~©åJE
Å©%
© e
¥PuJš\@û€4ºVd—l|WË:ùÛìÍ-ììŒ §6ƒáË=xœ! ŞÿŸü6ÚŸºyOäŠÿ0ßÌD÷»
â¤iF‘JG‘´k#ša¨xœ340031QHÎÏ+.IÌ+ÑKÏgXptëé3ÎnYòdåÉ?–ŠJŒ¬š&@ ZT”_Ä°yéÓ`õÿ+ıá3*ıûbÂ©ûÄ!ò9ùéé©E)†Ÿ÷0ÿ2y#öH÷‹WºKÄÛU ìÅ,¢í	ãexœ bÿ—ôí;J/ÙHJañÆf~Ì¿ûF“*:(GåÆ	ı×’4œ k£æ-ˆŞ40000 deployments c÷/î=&z±mµ¿E~İ“eo6ÙÒo§‡uÛÚ>‰³ÎóÇg100644 go.sum zr™­I|Zo7-9ıwb-T“
L“yGAJë	€,xœ› dÿô—íT<‹–$a€ëw²í×è8sˆr“*êÇƒ˜¯‚cŠg¾ñäÅ‰Ép Ü“?•6ıbEf ¦º<JÏ	…-èLºX100644 go.sum ü¢ü‘ÌÛ dÒG,õ_¨nàğ“
L#100644 main.go ÅW’ñS{Es¯¤—¹“òr¶w™“V©B½f‡¨bxœ›È˜1Q3 ~£xœ340031QÈMÌÌÓKÏgèğ·ŞvjöñTŞ5.±Åwş2%±oë  ÔW¡ºxœ}»Â †gÎS`§v(ì&]ÑÍíH)=‘[bÔôİ¥êìôß‹¨®h4wH€\)óX3¹ÜT1”çr*89Ò•úÓG’&ôT
}Ö.ZÌZbŒÿÏ‚½IŒÑ©`*^}vÛ¿€é”øvàµFìõ½íÄ±TŒ&¾¢ÍÀ=ÙÕÈê5qHä³õmEÕÃ¾µâŒQœŠÁTã;Ìh|ŞG¹J¶¥xœ31 …ÜÌô¢Ä’Ìü¼b†	\ßåÖDd:¾Ã¢¢9vAúÚµ óf—àƒpxœÀ ?ÿôÓŒr853ÔDÛĞ8…i2+D&Q²n‘ /LX“m“U²Ò¹ò—²'m‘ã$db ÀAë¥=•Cp|ñ¢5÷Î¢“N“?•6á~?û«Ô_l?FÛB÷Ï"100644 go.sum Ãå{òzÍ×@6‡o}7*ëB?.´“
82IÛ| rP: ©––˜âİÃ~“ò¨40000 pkg ›šWgİk0`â×ÿìã‘ ü¶!+õ“RYé€OxœÉ 6ÿÓôÏNŞ3jÂ¶K"$LÑêMƒï¾–‘ã$8config Ò"ìv¼Ãø˜J‚RIµm6F40000 db (GåÆ	ı×’4œ k£æ-ˆŞ“•6äZ šr7üK…·÷©$)ºÅv100644 go.sum 4ºT%!¯ÊuL†Â@6¦B“é82ön0æ£XW°Ü¼ •ÀY ò}i40000 pkg ¿ÓŒ¢¥™n½úšÕ[º¤ı1+Tøäˆ™axœ›®tCqC,ãäÙŒ’Æ‰)¹™yV`Ò!'?91'#¿¸ÄÊÔÄØH?=?>73¹(¿8µ¨,395¾$5· '±$uóF}~ hM¥xœ340031QHÎÏKËL×KÏgĞ¿ğr¾Yô£ÈĞ/÷«‹87^œµ¯B üu­áåfxœ»Ï;wBÈÄò3“·2ªNfê˜¼ƒÉub°Øä VÉÉ™
&/dÎ™œÎ(æó²$M¾ÅÈfç³LüQ Ìfd=1y6ßæu¬†Œ ÉiäVxœ›Æ»wBÈda&Ñ‰n·ÄÓó‹2srõ‹S‹‹3óóŠÊõLô&0+N.`›¬É(3y3ßdÆ©“ÃYù&ïc™ìÈT ×–\Z”šœŸŸ™
Òj8y"³ôäL¹“½˜“6_fnaÜÜÀ*Æ äú&Éç ãpxœ…Ğ¿Nƒ@ Çñuµ»;¦@á€š8Ğ*¶µUZl7ş\9zWî€¶K_ÁÍÍ7Ğhâ›8û(J\ÔÄ8ş–O¾ù½½n¼¿l<nn=ßnì®C<ÍW«ıB¬7êÂ~$.FKYçFÑµŸİôDÏœ–CÂ ­sÕpüvÇV ¯ö±p´Æ,Ê½ºS>Ä8Dÿá}®zŠƒŠZHh[)&<—:æX6±DÒñŞÍ"QŒ¡‰–wÄpŞİï=­q#äòúy}Œ“Vê§[qN»k÷,S¥LQEö(¸."6sT)j'†–\kĞÔUşÉ~‹=Ÿ¶×ºåùùÈr8ÉÕ²{%º¤1¹¤\•X$xá¦SüOiŒg´¢å¯_Òu.cepÊ`†Œ5šQ«l²Üğ˜¬ô™êæ«ù°"'ÿ™ßr¾{Ë¦ÈEH%|9Z³¨éÙVóI¿S_E— Nîjà9«ÆZa¯é€×RxœËÌœ*"²Uø¹}ÏSñğ†•	7Ôãº'm ˆŒ#¡xœ340031QÈO,-ÉˆOÎÏKËL/-J,É/ÒKÏgHsùâÏòÄ>±ı{`ùµI]ò9O– ¶¡ë®xœ31 …ô¢‚dû½ïCç¤øşTìqÿyôçì³GûLÀÒ%%‚Û×^[}ÈoR¬Ÿ´ÂufÏæ
ˆtvbZv"Ã§O‹-œOïJè»{0v“ƒèí'Êk_'¥¤xœ340031QÈO,-ÉˆO/*HOÎÏ+)ÊÏÉI-ÒKÏgHôÿİñm³ïÆÅğÍ–^}Fyî† ³'¢xœ340031QH,(ĞKÏgà>ıõû/KÙ»‹æ¯g¯kRûU8² Ø}3¥xœ31 …ÜÌô¢Ä’Ìü¼bƒÍ«-ÔxÖ»š]`ş¥×¶©o’‹ gZe‡Î7xœ»Ä^7¡ Sfd€ÚCxœËôœà	 ìŒ¤xœ340031Q(-ÉÌ)ÖKÏg8xêÿkù}›euöf[õYŸœø„ õèßeëí,ñåV«él1Zn¹n›

// .git/objects/pack/pack-8f65ebed1d2cf1e556abe96c315a170d6eb96e9b.rev
RIDX             ¿  ‡          Ú   Ó   e  |  â   –   }   w  Ñ   D   ¬  Z   Ô   "      (        à     ;   ¹   °  ~        Z     T   ä   c    +   <   Ä   •   `     q  ˆ  v   Ğ   ÿ  Ã  ¢   3  ù  •  µ  ¸     ^  T  i   2  …   A        d   †  g  !  ·   …  ¶  ?  ­  ©   ´  —   Ç   š  ¯    Ï  ‘   ²  G  ï  ¼  ‹  ô  a     ³  –  ğ     ¶   f   ¿   ç   _  "    ‚  ´   ê  Ê     $  N  Í  ê   Ø  t   Å  s      -   º        µ  ]   É  †  B  "   ‚   İ    Ø  8  ¬   Ò         £      Š   Q  ±         0  &  ”       I   Ş  O  }  Y  A  -    ƒ  Q  ‰    3   ]  Â   ò  x   *  ½  H   j  ÷   ±  4     a  W  q   ï  ^  f   ;     E   Ï   ‰  š   N  ª   ½   ‘        l   t   &   b  ,  <      H  ¡  =   ø  Ö   Î  @   J   ı   ˜  î  6   „  Ë  )   C  /    
   O  $  æ   ú     ƒ  ³  œ  ¥  ü   i   X   ¸   \   »   è  ë   ö      o  ó      û    %  ã   :      ¢  j      —   V   L        ¾  ¦  *  Ä   Á   À  °   W   ù  ú   8    D  «   Ÿ   ˆ     È   Ë  ¤  Û     §       p  E   9  '  b   ™    İ   )  Ì     2   P  §  `   +   6   U  Ç   ë   ğ       Ñ     G    C     º   
  	   ô  >  P   ã     \   ?   R     é   x   ¨  
  5  r      €   ×  è  á   Ã    ò   Y  V   ñ  o   @  À   ,      !     1   Ì   Õ   ó  Ÿ   Â   Û   ©   #     g  .  1  »     L  ˜  õ     à  $     ’   ‡  e      ¦   ~  U     |   Ú  £  0   u    J   ®  X  Î  {      p   õ      â     r  _   ¤  å   [   á   Œ  û  Ò  &   y  ä   n   5  ®   F  Ô  h   Æ     '   ì   K  ×  ş   ·  ™  Œ  È  z  é  y   ¼  “   =   /   “  É     k   s   >  ì  k   æ   7  S  ç   å  m     ¡  n     Ó  Ş  Æ       ¹    ›  u     ü  9   Í  Ù       4          œ   ›  ø  Ğ     v  Š    ö   M  „     ¾  ²   	   í  d  í  €     ­  '  Ü       	  #  Á   Ü   ş  %  Õ  (  ¨  ı  :   ¯  R     7  (   Ö   Ê   .  w  F   %         Ù     m     c  [       M   B   ‹  ÿ   î  I   ”    ß   «   z   ß   {  #   S   ’     ÷  Å   !  ñ   l               ª     h   ¥  Keëí,ñåV«él1Zn¹n›|Kpí9Ğd¾{5÷t
Ïâ¢?(ê

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
â”œâ”€â”€ .github
â”‚  â”œâ”€â”€ CODEOWNERS
â”‚  â””â”€â”€ pull_request_template.md
â”œâ”€â”€ app
â”‚  â””â”€â”€ app.go
â”œâ”€â”€ cmd
â”‚  â””â”€â”€ main.go
â”œâ”€â”€ db
â”‚  â””â”€â”€ migrations
â”‚    â””â”€â”€ time_migrate_name.up.sql
â”œâ”€â”€ envs
â”‚  â”œâ”€â”€ .env
â”‚  â”œâ”€â”€ local.env
â”‚  â”œâ”€â”€ production.env
â”‚  â”œâ”€â”€ stage.env
â”‚  â””â”€â”€ test.env
â”œâ”€â”€ external
â”‚  â””â”€â”€ service_name_module
â”‚    â”œâ”€â”€ domain
â”‚    â”œâ”€â”€ exception
â”‚    â”œâ”€â”€ dto
â”‚    â””â”€â”€ usecase
â”œâ”€â”€ internal
â”‚  â””â”€â”€ module_name
â”‚    â”œâ”€â”€ configurator
â”‚    â”œâ”€â”€ delivery
â”‚    â”‚  â”œâ”€â”€ grpc
â”‚    â”‚  â”œâ”€â”€ http
â”‚    â”‚  â””â”€â”€ kafka
â”‚    â”‚    â”œâ”€â”€ consumer
â”‚    â”‚    â””â”€â”€ producer
â”‚    â”œâ”€â”€ domain
â”‚    â”œâ”€â”€ dto
â”‚    â”œâ”€â”€ exception
â”‚    â”œâ”€â”€ job
â”‚    â”œâ”€â”€ repository
â”‚    â”œâ”€â”€ tests
â”‚    â”‚  â”œâ”€â”€ fixtures
â”‚    â”‚  â””â”€â”€ integrations
â”‚    â””â”€â”€ usecase
â”œâ”€â”€ pkg
â”‚  â”œâ”€â”€ client_container
â”‚  â”œâ”€â”€ config
â”‚  â”œâ”€â”€ constant
â”‚  â”œâ”€â”€ cron
â”‚  â”œâ”€â”€ env
â”‚  â”œâ”€â”€ error
â”‚  â”‚  â”œâ”€â”€ contracts
â”‚  â”‚  â”œâ”€â”€ custom_error
â”‚  â”‚  â”œâ”€â”€ error_utils
â”‚  â”‚  â”œâ”€â”€ grpc
â”‚  â”‚  â””â”€â”€ http
â”‚  â”œâ”€â”€ grpc
â”‚  â”œâ”€â”€ http
â”‚  â”œâ”€â”€ infra_container
â”‚  â”œâ”€â”€ kafka
â”‚  â”‚  â”œâ”€â”€ consumer
â”‚  â”‚  â””â”€â”€ producer
â”‚  â”œâ”€â”€ logger
â”‚  â”œâ”€â”€ postgres
â”‚  â”œâ”€â”€ redis
â”‚  â”œâ”€â”€ sentry
â”‚  â””â”€â”€ wrapper
â”‚
â”œâ”€â”€ .gitignore
â”œâ”€â”€ .pre-commit-config.yaml
â”œâ”€â”€ golangci.yaml
â”œâ”€â”€ docker-compose.e2e-local.yaml
â”œâ”€â”€ docker-compose.yaml
â”œâ”€â”€ go.sum
â”œâ”€â”€ Makefile
â”œâ”€â”€ go.mod
â”œâ”€â”€ LICENSE
â””â”€â”€ README.md
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


