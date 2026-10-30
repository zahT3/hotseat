import Testing
@testable import HotseatKit

@Test func refreshGateCoalescesAndReruns() {
    var g = RefreshGate()
    let first = g.request()   // idle: start now
    let second = g.request()  // running: remembered, not started
    let third = g.request()
    let rerun = g.finish()    // one rerun for the requests made meanwhile
    let idle = g.finish()     // nothing more pending
    let again = g.request()
    #expect([first, second, third, rerun, idle, again] == [true, false, false, true, false, true])
}
