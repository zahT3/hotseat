import Foundation

let panelFixture = """
{
  "generatedAt": 1793328100,
  "current": "claude3",
  "recommend": "claude2",
  "futureField": 1,
  "desktop": {"managed": true, "active": "claude3", "running": false, "runningKnown": true, "pendingRecovery": false},
  "accounts": [
    {"account": "claude", "current": false, "fiveHour": null, "sevenDay": null,
     "limit": {"message": "You've hit your weekly limit", "resetsAt": 1793527200, "observedAt": 1793253315, "sessionId": "s", "cwd": "/tmp"},
     "limitActive": true, "desktop": "parked"},
    {"account": "claude2", "current": false,
     "fiveHour": {"usedPercent": 0, "resetsAt": 1793309400, "observedAt": 1793293372, "source": "statusline", "reset": true},
     "sevenDay": {"usedPercent": 64, "resetsAt": 1793732400, "observedAt": 1793293401, "source": "statusline", "reset": false},
     "limit": null, "limitActive": false, "desktop": "parked", "desktopProblem": "not logged in yet"},
    {"account": "claude3", "current": true,
     "fiveHour": {"usedPercent": 20, "resetsAt": 1793345400, "observedAt": 1793328072, "source": "statusline", "reset": false},
     "sevenDay": {"usedPercent": 79, "resetsAt": 1793617200, "observedAt": 1793328046, "source": "desktop", "reset": false},
     "limit": null, "limitActive": false, "desktop": "active"}
  ]
}
"""
