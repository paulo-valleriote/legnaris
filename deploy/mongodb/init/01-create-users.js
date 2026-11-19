// Creates least-privileged users so services never connect as root. bootstrap.sh
// runs it authenticated as the root user, after the root user exists, on every
// stack start; a user that already exists keeps its password, so a restart never
// rotates it, but is granted any role added here since it was created.
//
//   legnaris_app       readWrite on `legnaris`         -> the Go API
//   legnaris_exporter  clusterMonitor on `admin`       -> mongodb-exporter
//   legnaris_view      read on `legnaris`,             -> Mongo Express
//                      clusterMonitor on `admin`          (index page runs serverStatus)
//
// Passwords come from the bootstrap container environment (see docker-compose.yml).

function ensureUser(database, spec) {
  if (database.getUser(spec.user) === null) {
    database.createUser(spec);
    return;
  }
  database.grantRolesToUser(spec.user, spec.roles);
}

const appDB = db.getSiblingDB("legnaris");

ensureUser(appDB, {
  user: "legnaris_app",
  pwd: process.env.MONGO_APP_PASSWORD,
  roles: [{ role: "readWrite", db: "legnaris" }],
});

ensureUser(appDB, {
  user: "legnaris_view",
  pwd: process.env.MONGO_VIEW_PASSWORD,
  roles: [
    { role: "read", db: "legnaris" },
    { role: "clusterMonitor", db: "admin" },
  ],
});

ensureUser(db.getSiblingDB("admin"), {
  user: "legnaris_exporter",
  pwd: process.env.MONGO_EXPORTER_PASSWORD,
  roles: [{ role: "clusterMonitor", db: "admin" }],
});
