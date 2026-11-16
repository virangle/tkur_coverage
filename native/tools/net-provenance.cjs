// SPDX-License-Identifier: EPL-2.0
// Developer helper: all paths and version evidence are explicit, no private tree.
'use strict';
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert');
const [measureArg,assemblyArg,hostArg,beforeHash,versionsArg]=process.argv.slice(2);
if(!versionsArg)throw Error('usage: MEASURE_DIR RESTORED_ASSEMBLY REPLAY_HOST_DLL BEFORE_SHA256 VERSIONS_JSON');
const dir=path.resolve(measureArg),assembly=path.resolve(assemblyArg),host=path.resolve(hostArg);
const hash=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const loaded=JSON.parse(fs.readFileSync(path.join(dir,'loaded.json'),'utf8'));
const versions=JSON.parse(fs.readFileSync(versionsArg,'utf8')); // Actual captured tool versions/command, never hardcoded successes.
const restored=hash(assembly);
assert.match(beforeHash,/^[a-f0-9]{64}$/);
assert.equal(restored,beforeHash,'Coverlet must restore original bytes');
assert.notEqual(loaded.loaded.sha256,restored,'replay must load instrumented bytes');
assert.equal(loaded.loaded.location,assembly);
fs.writeFileSync(path.join(dir,'provenance.json'),JSON.stringify({execution_owned_by:'external',producer:'coverlet.console',versions,raw_sha256:hash(path.join(dir,'coverage.json')),restored_assembly_sha256:restored,pdb_sha256:hash(assembly.replace(/\.dll$/i,'.pdb')),replay_host_sha256:hash(host),loaded_evidence:loaded,coverlet_log_sha256:hash(path.join(dir,'coverlet.log'))},null,2));
