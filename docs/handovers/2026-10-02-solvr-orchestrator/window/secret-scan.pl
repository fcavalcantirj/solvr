#!/usr/bin/perl
# Cutover window G1: secret scan over `git log -p` output. Prints ONLY masked tokens (prefix + 3 chars + ****).
# Output TSV: commit, file, new_line, sign(+/-/msg), pattern, masked, path_hint, value_hint
use strict; use warnings;
my @pats = (
  ['sk-',            qr/sk-[A-Za-z0-9_\-]{8,}/,                3],
  ['solvr_sk_',      qr/solvr_sk_[A-Za-z0-9_\-]{4,}/,          9],
  ['solvr_rm_',      qr/solvr_rm_[A-Za-z0-9_\-]{4,}/,          9],
  ['solvr_rt_',      qr/solvr_rt_[A-Za-z0-9_\-]{4,}/,          9],
  ['ghp_',           qr/ghp_[A-Za-z0-9]{8,}/,                  4],
  ['github_pat_',    qr/github_pat_[A-Za-z0-9_]{8,}/,          11],
  ['gsk_',           qr/gsk_[A-Za-z0-9]{8,}/,                  4],
  ['AKIA',           qr/AKIA[0-9A-Z]{12,}/,                    4],
  ['private_key',    qr/BEGIN [A-Z ]*PRIVATE KEY/,             -1],
  ['password',       qr/password["']?\s*[:=]\s*["']?[^\s"',;)]*/i, 0],
  ['pg_url_creds',   qr/postgres(?:ql)?:\/\/[^:\/@\s'"`]+:[^@\s'"`]+@[^\/:\s'"`?]+/, 0],
  ['x:solvr_key',    qr/\bsolvr_[A-Za-z0-9]{24,}/,             6],
  ['x:resend',       qr/\bre_[A-Za-z0-9]{20,}/,                3],
  ['x:voyage',       qr/\bpa-[A-Za-z0-9_\-]{20,}/,             3],
  ['x:jwt',          qr/eyJ[A-Za-z0-9_\-]{20,}\./,             3],
  ['x:named_secret', qr/(?:ADMIN_API_KEY|JWT_SECRET|SOLVR_DB_PASSWORD|SOLVR_DEPLOY_[A-Z_]+)["']?\s*[=:]\s*["']?[^\s"',;)]+/, 0],
);
my $ph = qr/(example|placeholder|xxx|dummy|test|fake|your|redacted|changeme|secret|\.\.\.|<|\$\{|\$[A-Za-z_(]|\*\*|abc|123|foo|bar|solvr_dev|getenv|process\.env|os\.)/i;
my ($commit, $file, $nl) = ('', '', 0);
while (my $l = <STDIN>) {
  chomp $l;
  if ($l =~ /^commit ([0-9a-f]{40})/) { $commit = substr($1, 0, 8); $file = '<msg>'; $nl = 0; next; }
  if ($l =~ /^diff --git a\/\S+ b\/(\S+)/) { $file = $1; next; }
  if ($l =~ /^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@/) { $nl = $1; next; }
  next if $l =~ /^(\+\+\+|---) /;
  my ($sign, $text, $line);
  if ($file eq '<msg>') { ($sign, $text, $line) = ('msg', $l, 0); }
  elsif ($l =~ /^\+(.*)$/) { ($sign, $text, $line) = ('+', $1, $nl); $nl++; }
  elsif ($l =~ /^-(.*)$/)  { ($sign, $text, $line) = ('-', $1, $nl); }
  elsif ($l =~ /^ (.*)$/)  { $nl++; next; }
  else { next; }
  for my $p (@pats) {
    my ($name, $re, $keep) = @$p;
    while ($text =~ /($re)/g) {
      my $m = $1; my ($masked, $val);
      if ($name eq 'pg_url_creds') {
        my ($host) = $m =~ /@([^\/:\s]+)$/;
        next if $host =~ /^(localhost|127\.0\.0\.1|postgres|db|host|HOST|\$.*|<.*)$/;
        my ($user, $pass) = $m =~ /:\/\/([^:]+):([^@]+)@/;
        $masked = 'postgres://' . substr($user,0,3) . '****:****@' . substr($host,0,3) . '****';
        $val = "$user:$pass";
      } elsif ($keep < 0) { $masked = $m; $val = ''; }
      elsif ($keep == 0) { my ($k) = $m =~ /^(.*?[:=]\s*["']?)/; $k //= ''; $val = substr($m, length($k)); $masked = $k . substr($val, 0, 3) . '****'; }
      else { $masked = substr($m, 0, $keep + 3) . '****'; $val = substr($m, $keep); }
      my $pathh = ($file =~ /(_test\.|\.test\.|\.spec\.|testdata|\/tests?\/|e2e|^docs\/|\.md$|example|fixture|mock)/i) ? 'test/docs-path' : '-';
      my $valh = ($val eq '' || $val =~ $ph) ? 'placeholder-like' : '-';
      print join("\t", $commit, $file, $line, $sign, $name, $masked, $pathh, $valh), "\n";
    }
  }
}
