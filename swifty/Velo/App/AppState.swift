import Foundation
import Observation

@Observable
final class AppState {
    var isAuthenticated: Bool = false
    var hasCompletedOnboarding: Bool = false
    var currentUser: User? = nil
    var sessions: [Session] = MockData.sessions
    var activeSessionIntercept: Bool = true

    var activeSession: Session? {
        sessions.first { $0.status == .active }
    }

    // Toast state
    var toastMessage: String? = nil

    var sessionsByDate: [Date: [Session]] {
        var result: [Date: [Session]] = [:]
        let calendar = Calendar.current
        for session in sessions {
            let day = calendar.startOfDay(for: session.createdAt)
            result[day, default: []].append(session)
        }
        return result
    }

    func login() {
        isAuthenticated = true
        currentUser = MockData.currentUser
    }

    func completeOnboarding(displayName: String) {
        hasCompletedOnboarding = true
        currentUser?.displayName = displayName
    }

    func signOut() {
        isAuthenticated = false
        hasCompletedOnboarding = false
        currentUser = nil
        sessions = []
    }

    func createSession(_ session: Session) {
        sessions.append(session)
    }

    func updateUserDisplayName(_ name: String) {
        currentUser?.displayName = name
        showToast("Profile updated")
    }

    func showToast(_ message: String) {
        toastMessage = message
        Task {
            try? await Task.sleep(for: .seconds(3))
            await MainActor.run { toastMessage = nil }
        }
    }

    func sessionsForDate(_ date: Date) -> [Session] {
        let calendar = Calendar.current
        let day = calendar.startOfDay(for: date)
        return sessions.filter { calendar.startOfDay(for: $0.createdAt) == day }
    }

    func datesWithSessions(in month: Date) -> Set<Date> {
        let calendar = Calendar.current
        let components = calendar.dateComponents([.year, .month], from: month)
        guard let monthStart = calendar.date(from: components),
              let monthEnd = calendar.date(byAdding: .month, value: 1, to: monthStart) else {
            return []
        }
        var result = Set<Date>()
        for session in sessions {
            let day = calendar.startOfDay(for: session.createdAt)
            if day >= monthStart && day < monthEnd {
                result.insert(day)
            }
        }
        return result
    }
}
