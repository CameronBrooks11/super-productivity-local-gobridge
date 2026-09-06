import subprocess, shutil, sys
SRC='internal/hostcfg/hostcfg.go'
orig=open(SRC).read()
GUARD='	if len(remaining) > 1 {'
MSG="len(remaining), remaining[1])"
FMT='"Error: expected one host, got %d (unexpected argument \'%s\')\\n"'
LOOP='			if !strings.HasPrefix(arg, "-") {'

def sub(old,new,which):
    s=orig
    assert s.count(old)>=1, ('NO MATCH',old[:40])
    i=s.find(old) if which==0 else s.rfind(old)
    r=s[:i]+new+s[i+len(old):]
    assert r!=s, 'NO CHANGE'
    return r

def move(target, which):
    """Relocate the guard block within one function."""
    s=orig
    blocks=[]
    idx=0
    while True:
        i=s.find(GUARD, idx)
        if i<0: break
        j=s.index('\t}\n', i)+3
        blocks.append((i,j)); idx=j
    assert len(blocks)==2
    i,j = blocks[which]
    block=s[i:j]
    s2 = s[:i]+s[j:]
    anchors={
      'help': ('	if wantHelp {', 'before'),
      'status': ('	if status {', 'before'),
      'lookup': ('		fmt.Fprintf(os.Stderr, "Supported hosts: %s\\n", strings.Join(sortedHostNames(), ", "))\n		return 2\n	}\n', 'after'),
    }
    anc,where = anchors[target]
    k = s2.find(anc) if which==0 else s2.rfind(anc)
    assert k>0, target
    return (s2[:k]+block+"\n"+s2[k:]) if where=='before' else (s2[:k+len(anc)]+"\n"+block+s2[k+len(anc):])

MUTS=[]
for w,tag in ((1,'cfg'),(0,'pc')):
    MUTS += [
      (tag,'exit 0 instead of 2',            lambda w=w: sub('		return 2\n	}\n\n	hostName := remaining[0]','		return 0\n	}\n\n	hostName := remaining[0]',w)),
      (tag,"don't reject extras",            lambda w=w: sub(GUARD,'	if false {',w)),
      (tag,'reject at >2 (off by one)',      lambda w=w: sub(GUARD,'	if len(remaining) > 2 {',w)),
      (tag,'name remaining[0]',              lambda w=w: sub('remaining[1])','remaining[0])',w)),
      (tag,'name the LAST extra',            lambda w=w: sub('remaining[1])','remaining[len(remaining)-1])',w)),
      (tag,'hardcode the count as 2',        lambda w=w: sub(MSG,'2, remaining[1])',w)),
      (tag,'count len(args)',                lambda w=w: sub(MSG,'len(args), remaining[1])',w)),
      (tag,'index args[1]',                  lambda w=w: sub(MSG,'len(remaining), args[1])',w)),
      (tag,'skip empty positionals in loop', lambda w=w: sub(LOOP,'			if arg != "" && !strings.HasPrefix(arg, "-") {',w)),
      (tag,'guard requires non-empty extra', lambda w=w: sub(GUARD,'	if len(remaining) > 1 && remaining[1] != "" {',w)),
      (tag,'guard requires non-flag extra',  lambda w=w: sub(GUARD,'	if len(remaining) > 1 && !strings.HasPrefix(remaining[1], "-") {',w)),
      (tag,'drop the "Error: " prefix',      lambda w=w: sub(FMT, FMT.replace('Error: ',''), w)),
      (tag,'unquote the argument',           lambda w=w: sub(FMT, FMT.replace("'%s'",'%s'), w)),
      (tag,'guard moved before --help',      lambda w=w: move('help', 1 if w else 0)),
      (tag,'guard moved before --status',    lambda w=w: move('status', 1 if w else 0)) if tag=='cfg' else None,
      (tag,'guard moved after host lookup',  lambda w=w: move('lookup', 1 if w else 0)),
      (tag,'reject at >0 (panics)',          lambda w=w: sub(GUARD,'	if len(remaining) > 0 {',w)),
    ]
MUTS=[m for m in MUTS if m]

rows=[]
for tag,name,fn in MUTS:
    try: mutated=fn()
    except AssertionError as e:
        rows.append((tag,name,'DID NOT APPLY')); continue
    open(SRC,'w').write(mutated)
    if subprocess.run(['go','build','./...'],capture_output=True).returncode!=0:
        rows.append((tag,name,'BUILD FAIL')); open(SRC,'w').write(orig); continue
    r=subprocess.run(['go','test','./internal/hostcfg/','-count=1','-v'],capture_output=True,text=True)
    out=r.stdout+r.stderr
    n=sum(1 for l in out.splitlines() if l.strip().startswith('--- FAIL'))
    rows.append((tag,name,'panic' if 'panic:' in out else ('SURVIVES' if n==0 else str(n))))
    open(SRC,'w').write(orig)
open(SRC,'w').write(orig)
w=max(len(n) for _,n,_ in rows)
for tag,name,res in rows:
    print(f"{tag:<4}{name:<{w+2}}{res}")
