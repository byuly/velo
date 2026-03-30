import Foundation

// MARK: - Enums

enum SessionStatus: String, Codable, CaseIterable {
    case active, generating, complete, failed, cancelled
}

enum SlotParticipationStatus: String, Codable {
    case recording, skipped
}

enum ParticipantStatus: String, Codable {
    case active, excluded
}

// MARK: - Core Models

struct User: Identifiable, Equatable, Hashable {
    var id: String
    var displayName: String
    var avatarUrl: String?
}

struct SessionSlot: Identifiable, Equatable, Hashable {
    var id: String
    var name: String
    var startsAt: String   // e.g. "06:00"
    var endsAt: String     // e.g. "10:00"
    var slotOrder: Int
    var status: SlotParticipationStatus?
}

struct Participant: Identifiable, Equatable, Hashable {
    var id: String
    var userId: String
    var displayName: String
    var joinedAt: Date
    var status: ParticipantStatus
}

struct Clip: Identifiable, Equatable, Hashable {
    var id: String
    var userId: String
    var slotId: String
    var durationMs: Int
    var recordedAt: Date
}

struct Session: Identifiable, Equatable, Hashable {
    var id: String
    var creatorId: String
    var name: String
    var status: SessionStatus
    var deadline: Date
    var sectionCount: Int
    var maxSectionDurationSeconds: Int
    var slots: [SessionSlot]
    var participants: [Participant]
    var clips: [Clip]
    var reelUrl: String?
    var completedAt: Date?
    var createdAt: Date

    var isExpiringSoon: Bool {
        guard let completedAt else { return false }
        let expiryDate = completedAt.addingTimeInterval(75 * 24 * 3600) // 75 days warning
        return Date() >= expiryDate
    }

    var timeUntilDeadline: String {
        let interval = deadline.timeIntervalSinceNow
        guard interval > 0 else { return "Expired" }
        let hours = Int(interval) / 3600
        let minutes = (Int(interval) % 3600) / 60
        if hours > 0 {
            return "\(hours)h \(minutes)m"
        }
        return "\(minutes)m"
    }
}
