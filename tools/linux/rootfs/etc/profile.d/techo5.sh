export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PS1='techo5:\w # '
export STORE=/store
# A password-guarded USB console logs itself out after ten idle minutes, so a
# cable left in doesn't leave a root shell open.
if [ -e /run/techo5/root_pw_on ] && [ "$(tty)" = /dev/ttyGS0 ]; then
	TMOUT=600
	readonly TMOUT
	export TMOUT
fi
