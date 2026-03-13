import Foundation

// MARK: - Mock Data

enum MockData {
    static let currentUser = User(
        id: "user-alex",
        displayName: "Alex",
        avatarUrl: nil
    )

    static let participants: [Participant] = [
        Participant(id: "p1", userId: "user-alex", displayName: "Alex", joinedAt: Date(), status: .active),
        Participant(id: "p2", userId: "user-sam", displayName: "Sam", joinedAt: Date(), status: .active),
        Participant(id: "p3", userId: "user-jordan", displayName: "Jordan", joinedAt: Date(), status: .active),
    ]

    static let slots: [SessionSlot] = [
        SessionSlot(id: "slot-morning", name: "Morning", startsAt: "06:00", endsAt: "10:00", slotOrder: 0, status: .recording),
        SessionSlot(id: "slot-midday", name: "Midday", startsAt: "10:00", endsAt: "14:00", slotOrder: 1, status: nil),
        SessionSlot(id: "slot-afternoon", name: "Afternoon", startsAt: "14:00", endsAt: "18:00", slotOrder: 2, status: nil),
        SessionSlot(id: "slot-evening", name: "Evening", startsAt: "18:00", endsAt: "22:00", slotOrder: 3, status: nil),
    ]

    static let sessions: [Session] = [
        Session(
            id: "session-1",
            creatorId: "user-alex",
            name: "Sunday Vibes",
            status: .active,
            deadline: Date().addingTimeInterval(6 * 3600),
            sectionCount: 4,
            maxSectionDurationSeconds: 15,
            slots: slots,
            participants: participants,
            clips: [
                Clip(id: "clip-1", userId: "user-alex", slotId: "slot-morning", durationMs: 12000, recordedAt: Date().addingTimeInterval(-7200))
            ],
            reelUrl: nil,
            completedAt: nil,
            createdAt: Calendar.current.date(byAdding: .day, value: 0, to: Date()) ?? Date()
        ),
        Session(
            id: "session-2",
            creatorId: "user-alex",
            name: "Saturday Adventures",
            status: .complete,
            deadline: Date().addingTimeInterval(-24 * 3600),
            sectionCount: 3,
            maxSectionDurationSeconds: 20,
            slots: [
                SessionSlot(id: "s2-slot-1", name: "Morning", startsAt: "06:00", endsAt: "10:00", slotOrder: 0, status: .recording),
                SessionSlot(id: "s2-slot-2", name: "Afternoon", startsAt: "14:00", endsAt: "18:00", slotOrder: 1, status: .recording),
                SessionSlot(id: "s2-slot-3", name: "Evening", startsAt: "18:00", endsAt: "22:00", slotOrder: 2, status: .recording),
            ],
            participants: Array(participants.prefix(2)),
            clips: [],
            reelUrl: "https://cdn.velo.app/reels/session-2.mp4",
            completedAt: Date().addingTimeInterval(-23 * 3600),
            createdAt: Calendar.current.date(byAdding: .day, value: -1, to: Date()) ?? Date()
        ),
        Session(
            id: "session-3",
            creatorId: "user-alex",
            name: "Chill Day",
            status: .complete,
            deadline: Date().addingTimeInterval(-72 * 3600),
            sectionCount: 2,
            maxSectionDurationSeconds: 30,
            slots: [
                SessionSlot(id: "s3-slot-1", name: "Midday", startsAt: "10:00", endsAt: "14:00", slotOrder: 0, status: .recording),
                SessionSlot(id: "s3-slot-2", name: "Evening", startsAt: "18:00", endsAt: "22:00", slotOrder: 1, status: .recording),
            ],
            participants: [participants[0]],
            clips: [],
            reelUrl: "https://cdn.velo.app/reels/session-3.mp4",
            completedAt: Date().addingTimeInterval(-71 * 3600),
            createdAt: Calendar.current.date(byAdding: .day, value: -3, to: Date()) ?? Date()
        ),
    ]

    static let presetSlots: [(name: String, start: String, end: String)] = [
        ("Morning", "06:00", "10:00"),
        ("Midday", "10:00", "14:00"),
        ("Afternoon", "14:00", "18:00"),
        ("Evening", "18:00", "22:00"),
        ("Night", "22:00", "02:00"),
    ]

    static let sectionDurationOptions = [10, 15, 20, 30]
}
