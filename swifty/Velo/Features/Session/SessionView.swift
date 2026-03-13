import SwiftUI

struct SessionView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.dismiss) private var dismiss
    let session: Session

    var body: some View {
        ScrollView {
            VStack(spacing: Tokens.Spacing.xl) {
                // Timer card
                VCard {
                    HStack {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Time remaining")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                            Text(session.timeUntilDeadline)
                                .font(.system(size: 32, weight: .bold, design: .monospaced))
                                .foregroundStyle(Tokens.Colors.foreground)
                        }
                        Spacer()
                        VBadge(
                            text: session.status.label,
                            variant: session.status.badgeVariant
                        )
                    }
                }

                // Participants section
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    sectionHeader("Participants", count: session.participants.count)

                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack(spacing: Tokens.Spacing.md) {
                            ForEach(session.participants) { participant in
                                ParticipantChip(participant: participant)
                            }
                            if session.participants.count < 4 {
                                InviteChip(sessionId: session.id)
                            }
                        }
                        .padding(.horizontal, Tokens.Spacing.base)
                    }
                    .padding(.horizontal, -Tokens.Spacing.base)
                }

                // Slots section
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    sectionHeader("Time Slots", count: session.slots.count)

                    ForEach(session.slots) { slot in
                        SlotCard(slot: slot, session: session, clips: clipsForSlot(slot))
                    }
                }
            }
            .padding(.horizontal, Tokens.Spacing.base)
            .padding(.vertical, Tokens.Spacing.base)
        }
        .background(Tokens.Colors.backgroundSubtle.ignoresSafeArea())
        .navigationTitle(session.name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .navigationBarTrailing) {
                Button {
                    // Share invite link
                    let url = "https://velo.app/join/\(session.id.prefix(8))"
                    let activity = UIActivityViewController(activityItems: [url], applicationActivities: nil)
                    if let window = UIApplication.shared.connectedScenes
                        .compactMap({ $0 as? UIWindowScene })
                        .first?.windows.first {
                        window.rootViewController?.present(activity, animated: true)
                    }
                } label: {
                    Image(systemName: "square.and.arrow.up")
                        .font(.system(size: 15, weight: .medium))
                }
            }
        }
    }

    private func sectionHeader(_ title: String, count: Int) -> some View {
        HStack {
            Text(title.uppercased())
                .font(Tokens.Typography.label)
                .foregroundStyle(Tokens.Colors.mutedForeground)
                .kerning(0.5)
            Spacer()
            Text("\(count)")
                .font(Tokens.Typography.caption)
                .foregroundStyle(Tokens.Colors.mutedForeground)
        }
    }

    private func clipsForSlot(_ slot: SessionSlot) -> [Clip] {
        session.clips.filter { $0.slotId == slot.id }
    }
}

// MARK: - Participant Chip

private struct ParticipantChip: View {
    let participant: Participant

    var body: some View {
        VStack(spacing: Tokens.Spacing.xs) {
            VAvatar(displayName: participant.displayName, size: 44)
            Text(participant.displayName)
                .font(Tokens.Typography.caption)
                .fontWeight(.medium)
                .foregroundStyle(Tokens.Colors.foreground)
                .lineLimit(1)
        }
        .frame(width: 56)
    }
}

// MARK: - Invite Chip

private struct InviteChip: View {
    let sessionId: String
    @State private var copied = false

    var body: some View {
        Button {
            let url = "https://velo.app/join/\(sessionId.prefix(8))"
            UIPasteboard.general.string = url
            copied = true
            Task {
                try? await Task.sleep(for: .seconds(2))
                await MainActor.run { copied = false }
            }
        } label: {
            VStack(spacing: Tokens.Spacing.xs) {
                VInviteCircle(size: 44)
                Text(copied ? "Copied!" : "Invite")
                    .font(Tokens.Typography.caption)
                    .fontWeight(.medium)
                    .foregroundStyle(copied ? Tokens.Colors.success : Tokens.Colors.mutedForeground)
            }
            .frame(width: 56)
        }
        .buttonStyle(.plain)
    }
}

// MARK: - Slot Card

private struct SlotCard: View {
    let slot: SessionSlot
    let session: Session
    let clips: [Clip]

    @Environment(AppState.self) private var appState

    private var totalDurationMs: Int { clips.reduce(0) { $0 + $1.durationMs } }
    private var maxDurationMs: Int { session.maxSectionDurationSeconds * 1000 }
    private var progress: Double {
        guard maxDurationMs > 0 else { return 0 }
        return min(1.0, Double(totalDurationMs) / Double(maxDurationMs))
    }
    private var isRecorded: Bool { !clips.isEmpty }
    private var isSkipped: Bool { slot.status == .skipped }

    var body: some View {
        VCard {
            VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(slot.name)
                            .font(Tokens.Typography.h3)
                            .foregroundStyle(Tokens.Colors.foreground)
                        Text("\(slot.startsAt) – \(slot.endsAt)")
                            .font(Tokens.Typography.bodySmall)
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                    }
                    Spacer()
                    slotStatusBadge
                }

                if isRecorded {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack {
                            Text("\(totalDurationMs / 1000)s / \(session.maxSectionDurationSeconds)s")
                                .font(Tokens.Typography.caption)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                            Spacer()
                            Text("\(clips.count) clip\(clips.count == 1 ? "" : "s")")
                                .font(Tokens.Typography.caption)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                        }
                        VProgressBar(progress: progress)
                    }
                }

                if !isSkipped && session.status == .active {
                    HStack(spacing: Tokens.Spacing.sm) {
                        NavigationLink(value: NavDestination.camera(slot, session)) {
                            HStack(spacing: 6) {
                                Image(systemName: isRecorded ? "plus.circle" : "video.fill")
                                    .font(.system(size: 13))
                                Text(isRecorded ? "Add clip" : "Record")
                                    .font(.system(size: 13, weight: .semibold))
                            }
                            .foregroundStyle(Tokens.Colors.primaryForeground)
                            .frame(maxWidth: .infinity)
                            .frame(height: 36)
                            .background(Tokens.Colors.primary)
                            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
                        }
                        .buttonStyle(.plain)

                        if !isRecorded {
                            Button {
                                // Skip action (mock)
                                appState.showToast("Slot skipped")
                            } label: {
                                Text("Skip")
                                    .font(.system(size: 13, weight: .semibold))
                                    .foregroundStyle(Tokens.Colors.mutedForeground)
                                    .frame(maxWidth: .infinity)
                                    .frame(height: 36)
                                    .background(Tokens.Colors.muted)
                                    .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
            }
        }
    }

    private var slotStatusBadge: some View {
        Group {
            if isSkipped {
                VBadge(text: "Skipped", variant: .neutral)
            } else if isRecorded {
                VBadge(text: "Recorded", variant: .complete)
            } else {
                VBadge(text: "Pending", variant: .neutral)
            }
        }
    }
}

#Preview {
    NavigationStack {
        SessionView(session: MockData.sessions[0])
    }
    .environment(AppState())
}
