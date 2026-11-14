// Initiates the single-member replica set rs0 and waits for that member to become
// PRIMARY, so transactions have a primary to write to (ADR 0013). bootstrap.sh runs
// it first, unauthenticated, over the localhost exception; it runs before any user
// exists and never after the root user is created.
//
// Idempotent: a member that already reports the set name is left as it is.
// MONGO_RS_MEMBER is the host:port the member is reached at. The replica-set
// configuration must name that same address, or the driver rediscovers a
// different host and fails, so each stack sets it to the address its clients use.

const RS_NAME = "rs0";
const member = process.env.MONGO_RS_MEMBER || "localhost:27017";

if (db.hello().setName !== RS_NAME) {
  rs.initiate({ _id: RS_NAME, members: [{ _id: 0, host: member }] });
}

for (let attempt = 0; attempt < 60 && !db.hello().isWritablePrimary; attempt++) {
  sleep(1000);
}
if (!db.hello().isWritablePrimary) {
  throw new Error(`replica set ${RS_NAME} has no primary after 60s`);
}
