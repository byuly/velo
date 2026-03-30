import SwiftUI

struct CalendarGridView: View {
    @Binding var selectedDate: Date
    var sessionDates: Set<Date>
    var onDateSelected: (Date) -> Void

    @State private var displayedMonth: Date = Calendar.current.startOfMonth(for: Date())

    private let calendar = Calendar.current
    private let columns = Array(repeating: GridItem(.flexible(), spacing: 0), count: 7)
    private let dayLabels = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"]

    var body: some View {
        VStack(spacing: Tokens.Spacing.md) {
            // Month navigation header
            HStack {
                Button {
                    displayedMonth = calendar.date(byAdding: .month, value: -1, to: displayedMonth) ?? displayedMonth
                } label: {
                    Image(systemName: "chevron.left")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .frame(width: 36, height: 36)
                        .background(Tokens.Colors.muted)
                        .clipShape(Circle())
                }

                Spacer()

                Text(monthTitle)
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundStyle(Tokens.Colors.foreground)

                Spacer()

                Button {
                    displayedMonth = calendar.date(byAdding: .month, value: 1, to: displayedMonth) ?? displayedMonth
                } label: {
                    Image(systemName: "chevron.right")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .frame(width: 36, height: 36)
                        .background(Tokens.Colors.muted)
                        .clipShape(Circle())
                }
            }

            // Day-of-week labels
            HStack(spacing: 0) {
                ForEach(dayLabels, id: \.self) { label in
                    Text(label)
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .frame(maxWidth: .infinity)
                }
            }

            // Date grid
            LazyVGrid(columns: columns, spacing: 4) {
                ForEach(gridDays, id: \.self) { date in
                    if let date {
                        DayCell(
                            date: date,
                            isToday: calendar.isDateInToday(date),
                            isSelected: calendar.isDate(date, inSameDayAs: selectedDate),
                            hasSession: sessionDates.contains(calendar.startOfDay(for: date))
                        )
                        .onTapGesture {
                            selectedDate = date
                            onDateSelected(date)
                        }
                    } else {
                        Color.clear
                            .frame(height: 40)
                    }
                }
            }
        }
    }

    private var monthTitle: String {
        let formatter = DateFormatter()
        formatter.dateFormat = "MMMM yyyy"
        return formatter.string(from: displayedMonth)
    }

    private var gridDays: [Date?] {
        guard let monthInterval = calendar.dateInterval(of: .month, for: displayedMonth) else { return [] }
        let firstWeekday = calendar.component(.weekday, from: monthInterval.start)
        let leadingEmpties = firstWeekday - 1 // Sunday = 1, so 0 empties for Sunday start

        var days: [Date?] = Array(repeating: nil, count: leadingEmpties)

        var current = monthInterval.start
        while current < monthInterval.end {
            days.append(current)
            current = calendar.date(byAdding: .day, value: 1, to: current) ?? current
        }

        // Pad to complete last row
        let remainder = days.count % 7
        if remainder != 0 {
            days += Array(repeating: nil, count: 7 - remainder)
        }

        return days
    }
}

// MARK: - Day Cell

private struct DayCell: View {
    let date: Date
    let isToday: Bool
    let isSelected: Bool
    let hasSession: Bool

    private var dayNumber: String {
        let formatter = DateFormatter()
        formatter.dateFormat = "d"
        return formatter.string(from: date)
    }

    var body: some View {
        VStack(spacing: 2) {
            ZStack {
                if isSelected {
                    Circle()
                        .fill(Tokens.Colors.primary)
                        .frame(width: 34, height: 34)
                } else if isToday {
                    Circle()
                        .stroke(Tokens.Colors.primary, lineWidth: 1.5)
                        .frame(width: 34, height: 34)
                }

                Text(dayNumber)
                    .font(.system(size: 14, weight: isToday || isSelected ? .semibold : .regular))
                    .foregroundStyle(
                        isSelected ? .white :
                        isToday ? Tokens.Colors.primary :
                        Tokens.Colors.foreground
                    )
            }

            // Session dot
            Circle()
                .fill(isSelected ? Color.white.opacity(0.7) : Tokens.Colors.primary)
                .frame(width: 4, height: 4)
                .opacity(hasSession ? 1 : 0)
        }
        .frame(height: 44)
        .contentShape(Rectangle())
    }
}

// MARK: - Calendar Extension

extension Calendar {
    func startOfMonth(for date: Date) -> Date {
        let components = dateComponents([.year, .month], from: date)
        return self.date(from: components) ?? date
    }
}

#Preview {
    @Previewable @State var selected = Date()
    let calendar = Calendar.current
    let sessionDates: Set<Date> = [
        calendar.startOfDay(for: Date()),
        calendar.startOfDay(for: Date().addingTimeInterval(-86400)),
        calendar.startOfDay(for: Date().addingTimeInterval(-3 * 86400)),
    ]

    CalendarGridView(selectedDate: $selected, sessionDates: sessionDates) { _ in }
        .padding()
}
