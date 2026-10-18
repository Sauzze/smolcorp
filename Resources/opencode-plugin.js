// Smolcorp's OpenCode plugin. OpenCode has no command hooks, so this
// passes its session events to the hook the other agents run, named the way
// Claude Code names its hooks. The hook finds the session's tab and reports
// to the app; Smolcorp writes this file when you connect your agents,
// and keeps it up to date.
//
// Written for OpenCode 2, and wherever the version can't be told: OpenCode 2
// takes a default with an id and setup(), and leaves server() alone, while
// OpenCode 1.3 and later take its id and server(), and leave setup() alone.
// OpenCode 1 gets opencode-plugin-v1.js, which every version of it loads.
import { spawn } from "node:child_process"

const COMMAND = ["{{COMMAND}}"]
// How long OpenCode 2's event stream is left before it's read again, after
// it failed or ended while OpenCode carried on.
const RETRY_MS = 5000

// A session's events, from either OpenCode, reported as hook events.
function reporter(fallbackFolder) {
  // Subagent sessions run inside their parent's tab and are not its state.
  const children = new Set()
  // How a turn ended when it didn't finish. OpenCode follows an interrupt or
  // an error with the same session.idle as a finished turn.
  const endings = new Map()
  // Each session's folder, as it started in: the app finds its tab by it when
  // OpenCode runs plugins in a service of its own, away from the tab.
  const folders = new Map()

  const report = (sessionID, name, folder, extra = {}) => {
    if (!sessionID || children.has(sessionID)) return
    const cwd = folder ?? folders.get(sessionID) ?? fallbackFolder
    const event = JSON.stringify({ session_id: sessionID, hook_event_name: name, cwd, ...extra })
    // The hook gone, as when Smolcorp was moved or removed and this file left
    // behind, must not fail OpenCode's events: the deck just doesn't hear them.
    try {
      const hook = spawn(COMMAND[0], COMMAND.slice(1), { stdio: ["pipe", "ignore", "ignore"] })
      hook.on("error", () => {})
      hook.stdin.on("error", () => {})
      hook.stdin.end(event)
    } catch {}
  }

  return {
    started(sessionID, parentID, folder) {
      if (parentID) return children.add(sessionID)
      if (folder) folders.set(sessionID, folder)
      report(sessionID, "SessionStart", folder, { source: "startup" })
    },
    busy(sessionID, folder) {
      endings.delete(sessionID)
      report(sessionID, "UserPromptSubmit", folder)
    },
    ended(sessionID, ending, folder) {
      endings.set(sessionID, ending)
      report(sessionID, ending, folder)
    },
    idle(sessionID, folder) {
      report(sessionID, endings.get(sessionID) ?? "Stop", folder)
      endings.delete(sessionID)
    },
    asking: (sessionID, folder) => report(sessionID, "PermissionRequest", folder),
    answered: (sessionID, folder) => report(sessionID, "PostToolUse", folder),
    gone(sessionID, folder) {
      report(sessionID, "SessionEnd", folder)
      folders.delete(sessionID)
    },
  }
}

export default {
  id: "smolcorp",

  // OpenCode 1: an event hook, each event's details under properties.
  server: async ({ directory }) => {
    const session = reporter(directory)
    return {
      event: async ({ event }) => {
        const p = event.properties ?? {}
        switch (event.type) {
          case "session.created":
            session.started(p.info?.id, p.info?.parentID, directory)
            break
          case "session.status":
            if (p.status?.type === "busy") session.busy(p.sessionID)
            break
          case "session.idle":
            session.idle(p.sessionID)
            break
          case "permission.asked":
          case "permission.updated": // its name in older versions of OpenCode
            session.asking(p.sessionID)
            break
          case "permission.replied":
            session.answered(p.sessionID)
            break
          case "session.error":
            session.ended(p.sessionID, p.error?.name === "MessageAbortedError" ? "Interrupt" : "StopFailure")
            break
          case "session.deleted":
            session.gone(p.info?.id)
            break
        }
      },
    }
  },

  // OpenCode 2: a stream of events, each event's details under data, and
  // the folder it happened in under location. Run as a service, OpenCode
  // has these for every session it runs, whichever folder each is in. A
  // session shows itself with its first turn, which starts and ends as an
  // execution: OpenCode 2 doesn't send plugins session.created or
  // session.idle, so whether it's a subagent's is asked of OpenCode then.
  // The stream is read without setup waiting on it, again after a pause if
  // it fails or ends, and stops on cleanup.
  setup: async ({ location, event: events, session: sessions }) => {
    const session = reporter(location?.directory)
    const stop = new AbortController()
    const known = new Set()
    const seen = async (sessionID, folder) => {
      if (!sessionID || known.has(sessionID)) return
      known.add(sessionID)
      let parentID
      try {
        parentID = (await sessions?.get?.({ sessionID }))?.parentID
      } catch {}
      session.started(sessionID, parentID, folder)
    }
    const handle = async (event) => {
      const d = event.data ?? {}
      const folder = event.location?.directory
      switch (event.type) {
        case "session.created":
          known.add(d.sessionID)
          session.started(d.sessionID, d.parentID, d.location?.directory ?? folder)
          break
        case "session.execution.started":
          await seen(d.sessionID, folder)
          session.busy(d.sessionID, folder)
          break
        case "session.execution.succeeded":
          session.idle(d.sessionID, folder)
          break
        case "session.execution.interrupted":
          session.ended(d.sessionID, d.reason === "user" ? "Interrupt" : "Stop", folder)
          session.idle(d.sessionID, folder)
          break
        case "session.execution.failed":
          session.ended(d.sessionID, "StopFailure", folder)
          session.idle(d.sessionID, folder)
          break
        case "permission.asked":
          await seen(d.sessionID, folder)
          session.asking(d.sessionID, folder)
          break
        case "permission.replied":
          session.answered(d.sessionID, folder)
          break
        case "session.deleted":
          known.delete(d.sessionID)
          session.gone(d.sessionID, folder)
          break
      }
    }
    ;(async () => {
      while (!stop.signal.aborted) {
        try {
          for await (const event of events.subscribe({ signal: stop.signal })) {
            await handle(event).catch((error) => console.error("smolcorp: an event went unreported:", error))
          }
        } catch (error) {
          if (!stop.signal.aborted) console.error("smolcorp: OpenCode's events stopped coming:", error)
        }
        if (!stop.signal.aborted) await new Promise((resume) => setTimeout(resume, RETRY_MS))
      }
    })()
    return () => stop.abort()
  },
}
